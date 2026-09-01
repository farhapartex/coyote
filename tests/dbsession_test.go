package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/store"
)

func persistentApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	base := func(s *settings.Settings) { s.Sessions.Backend = settings.SessionsInDB }
	a := app.NewFrom(devSettings(t, append([]func(*settings.Settings){base}, fns...)...))
	syncSchema(t, a)
	return a
}

func TestDatabaseBackendRegistersTheSessionsTable(t *testing.T) {
	a := persistentApp(t)

	registered := false
	for _, m := range a.Models() {
		if _, ok := m.Entity.(session.Record); ok {
			registered = true
		}
	}
	if !registered {
		t.Error("the sessions model should be registered so makemigrations generates it")
	}

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	if !handle.Migrator().HasTable(&session.Record{}) {
		t.Fatal("the sessions table should be part of the schema")
	}
	for _, column := range []string{"id", "user_id", "data", "created_at", "expires_at"} {
		if !handle.Migrator().HasColumn(&session.Record{}, column) {
			t.Errorf("missing column %q", column)
		}
	}
}

func TestMemoryBackendDoesNotCreateASessionsTable(t *testing.T) {
	a := newTestApp(t)
	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	if handle.Migrator().HasTable(&session.Record{}) {
		t.Error("the memory backend should not migrate a sessions table")
	}
}

func TestSessionsSurviveARestart(t *testing.T) {
	a := persistentApp(t)
	a.Get("/count", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		sess.Set("visits", sess.GetInt("visits")+1)
		sess.Set("who", "jane")
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/count", nil))
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie was set")
	}

	restarted := app.NewFrom(a.Settings)
	restarted.Get("/count", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		sess.Set("visits", sess.GetInt("visits")+1)
		w.Write([]byte(sess.GetString("who")))
	})

	req := httptest.NewRequest(http.MethodGet, "/count", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	restarted.Handler().ServeHTTP(rec, req)

	if rec.Body.String() != "jane" {
		t.Errorf("values did not survive the restart, got %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/count", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	var visits int
	third := app.NewFrom(a.Settings)
	third.Get("/count", func(w http.ResponseWriter, r *http.Request) {
		visits = session.FromRequest(r).GetInt("visits")
	})
	third.Handler().ServeHTTP(rec, req)
	if visits != 2 {
		t.Errorf("visits = %d, want 2 after two increments across processes", visits)
	}
}

func TestGoTypesRoundTripThroughTheDatabase(t *testing.T) {
	handle := newTestDB(t)
	if err := migrate.Sync(handle, []model.Model{model.Of(session.Record{})}); err != nil {
		t.Fatal(err)
	}
	backend := store.Sessions(handle, 0)

	original := session.Restore("round-trip", nil, time.Now(), time.Now().Add(time.Hour))
	original.Set("count", 42)
	original.Set("ratio", 1.5)
	original.Set("flag", true)
	original.Set("name", "jane")
	original.Set("tags", []string{"a", "b"})
	original.SetUserID("user-1")
	original.AddFlash("success", "saved")

	if err := backend.Save(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	loaded, ok := backend.Load(t.Context(), "round-trip")
	if !ok {
		t.Fatal("session did not load")
	}

	if got := loaded.GetInt("count"); got != 42 {
		t.Errorf("GetInt = %d, want 42 — an int must not decay to a float", got)
	}
	if got := loaded.GetBool("flag"); !got {
		t.Error("GetBool did not round trip")
	}
	if got := loaded.GetString("name"); got != "jane" {
		t.Errorf("GetString = %q", got)
	}
	if tags, ok := loaded.Get("tags").([]string); !ok || len(tags) != 2 {
		t.Errorf("a []string should round trip as itself, got %T", loaded.Get("tags"))
	}
	if loaded.UserID() != "user-1" {
		t.Errorf("UserID = %q", loaded.UserID())
	}
	flashes := loaded.Flashes()
	if len(flashes) != 1 || flashes[0].Message != "saved" || flashes[0].Kind != "success" {
		t.Errorf("flashes did not round trip: %+v", flashes)
	}
}

func TestDatabaseStoreImplementsManageable(t *testing.T) {
	handle := newTestDB(t)
	if err := migrate.Sync(handle, []model.Model{model.Of(session.Record{})}); err != nil {
		t.Fatal(err)
	}
	backend := store.Sessions(handle, 0)

	for _, id := range []string{"a", "b", "c"} {
		s := session.Restore(id, nil, time.Now(), time.Now().Add(time.Hour))
		if id != "c" {
			s.SetUserID("u1")
		}
		if err := backend.Save(t.Context(), s); err != nil {
			t.Fatal(err)
		}
	}

	if sessionCount(t, backend) != 3 {
		t.Errorf("Count = %d, want 3", sessionCount(t, backend))
	}
	if len(allSessions(t, backend)) != 3 {
		t.Errorf("All returned %d", len(allSessions(t, backend)))
	}
	n, err := backend.DeleteByUserID(t.Context(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("DeleteByUserID = %d, want 2", n)
	}
	if sessionCount(t, backend) != 1 {
		t.Errorf("Count after delete = %d, want 1", sessionCount(t, backend))
	}
	if err := backend.Delete(t.Context(), "c"); err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.Load(t.Context(), "c"); ok {
		t.Error("deleted session should not load")
	}
}

func TestDatabaseStoreHidesExpiredSessions(t *testing.T) {
	handle := newTestDB(t)
	if err := migrate.Sync(handle, []model.Model{model.Of(session.Record{})}); err != nil {
		t.Fatal(err)
	}
	backend := store.Sessions(handle, 0)

	expired := session.Restore("gone", nil, time.Now().Add(-time.Hour), time.Now().Add(-time.Minute))
	if err := backend.Save(t.Context(), expired); err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.Load(t.Context(), "gone"); ok {
		t.Error("an expired session must not load")
	}
	if sessionCount(t, backend) != 0 {
		t.Errorf("expired sessions should not be counted, got %d", sessionCount(t, backend))
	}
	if len(allSessions(t, backend)) != 0 {
		t.Error("expired sessions should not be listed")
	}
}

func TestSavingTwiceUpdatesRatherThanDuplicates(t *testing.T) {
	handle := newTestDB(t)
	if err := migrate.Sync(handle, []model.Model{model.Of(session.Record{})}); err != nil {
		t.Fatal(err)
	}
	backend := store.Sessions(handle, 0)

	s := session.Restore("same-id", nil, time.Now(), time.Now().Add(time.Hour))
	s.Set("stage", "first")
	if err := backend.Save(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	s.Set("stage", "second")
	if err := backend.Save(t.Context(), s); err != nil {
		t.Fatalf("a second save must upsert, not fail: %v", err)
	}

	loaded, ok := backend.Load(t.Context(), "same-id")
	if !ok {
		t.Fatal("session missing")
	}
	if got := loaded.GetString("stage"); got != "second" {
		t.Errorf("stage = %q, want second", got)
	}
	if sessionCount(t, backend) != 1 {
		t.Errorf("saving twice created %d rows", sessionCount(t, backend))
	}
}

func TestLoginAndLogoutAcrossProcesses(t *testing.T) {
	a := persistentApp(t)
	if _, err := a.Auth.CreateSuperadmin("root", "", "supersecret"); err != nil {
		t.Fatal(err)
	}
	a.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		user, err := a.Auth.Authenticate("root", "supersecret")
		if err != nil {
			t.Fatal(err)
		}
		if err := a.Auth.Login(r, user); err != nil {
			t.Fatal(err)
		}
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookie := rec.Result().Cookies()[0]

	restarted := app.NewFrom(a.Settings)
	restarted.Get("/me", func(w http.ResponseWriter, r *http.Request) {
		if user := restarted.Auth.CurrentUser(r); user != nil {
			w.Write([]byte(user.Username))
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	restarted.Handler().ServeHTTP(rec, req)

	if rec.Body.String() != "root" {
		t.Errorf("the login should survive a restart, got %q", rec.Body.String())
	}
}

func TestSessionBackendValidation(t *testing.T) {
	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.Backend = "redis"
	})...); err == nil {
		t.Error("an unknown backend should be rejected")
	} else {
		mustContain(t, problemsOf(t, err), "Sessions.Backend")
	}

	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.Backend = settings.SessionsInDB
		s.Databases = nil
	})...); err == nil {
		t.Error("the database backend without a database should be rejected")
	}

	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.Backend = settings.SessionsInDB
	})...); err != nil {
		t.Errorf("the database backend with a database should validate: %v", err)
	}
}

func TestMemoryRemainsTheDefault(t *testing.T) {
	resolved, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Sessions.Backend != settings.SessionsInMemory {
		t.Errorf("Backend = %q, want memory by default", resolved.Sessions.Backend)
	}
}

func TestAnonymousRequestsDoNotFillTheSessionsTable(t *testing.T) {
	a := persistentApp(t)
	a.Get("/public", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	for range 3 {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public", nil))
	}

	sessions, ok := a.ManageableSessions()
	if !ok {
		t.Fatal("the database backend should be manageable")
	}
	if count := sessionCount(t, sessions); count != 0 {
		t.Errorf("a request that never touches the session should not be stored, got %d rows", count)
	}
}

func TestADatabaseSessionIsWrittenEvenWhenTheClientDisconnects(t *testing.T) {
	handle := newTestDB(t)
	if err := migrate.Sync(handle, []model.Model{model.Of(session.Record{})}); err != nil {
		t.Fatal(err)
	}
	backend := store.Sessions(handle, 0)
	m := session.NewManager(session.Options{Store: backend, Lifetime: time.Hour})

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("user", "jane")
		cancel()
	})).ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie was written")
	}
	restored, ok := backend.Load(t.Context(), cookies[0].Value)
	if !ok {
		t.Fatal("the session was lost because the request context had been cancelled")
	}
	if got := restored.GetString("user"); got != "jane" {
		t.Errorf("user = %q, want jane", got)
	}
}

func TestADatabaseSessionReadHonoursTheRequestContext(t *testing.T) {
	handle := newTestDB(t)
	if err := migrate.Sync(handle, []model.Model{model.Of(session.Record{})}); err != nil {
		t.Fatal(err)
	}
	backend := store.Sessions(handle, 0)

	live := session.Restore("live", nil, time.Now(), time.Now().Add(time.Hour))
	if err := backend.Save(t.Context(), live); err != nil {
		t.Fatal(err)
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, ok := backend.Load(cancelled, "live"); ok {
		t.Error("a read on a cancelled context should not reach the database")
	}
	if _, ok := backend.Load(t.Context(), "live"); !ok {
		t.Error("the session should still be readable on a live context")
	}
}
