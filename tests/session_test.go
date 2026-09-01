package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/view"
)

func TestSessionPersistsAcrossRequests(t *testing.T) {
	m := newTestManager()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		s.Set("count", s.GetInt("count")+1)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].HttpOnly {
		t.Error("cookie should be HttpOnly")
	}

	for want := 2; want <= 3; want++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookies[0])
		rec := httptest.NewRecorder()
		var got int
		m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := session.FromRequest(r)
			s.Set("count", s.GetInt("count")+1)
			got = s.GetInt("count")
		})).ServeHTTP(rec, req)
		if got != want {
			t.Errorf("count = %d, want %d", got, want)
		}
	}
}

func TestRenewRotatesIDAndKeepsValues(t *testing.T) {
	m := newTestManager()
	store := m.Store()

	rec := httptest.NewRecorder()
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("user", "jane")
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	first := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(first)
	rec = httptest.NewRecorder()
	var kept string
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.Renew(r); err != nil {
			t.Fatal(err)
		}
		kept = session.FromRequest(r).GetString("user")
	})).ServeHTTP(rec, req)

	second := rec.Result().Cookies()[0]
	if second.Value == first.Value {
		t.Error("session id was not rotated")
	}
	if kept != "jane" {
		t.Errorf("values lost on renew: %q", kept)
	}
	if _, ok := store.Load(t.Context(), first.Value); ok {
		t.Error("old session should be evicted from the store")
	}
	if _, ok := store.Load(t.Context(), second.Value); !ok {
		t.Error("new session should be in the store")
	}
}

func TestDestroyClearsCookieAndStore(t *testing.T) {
	m := newTestManager()

	rec := httptest.NewRecorder()
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("user", "jane")
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.Destroy(r); err != nil {
			t.Fatal(err)
		}
	})).ServeHTTP(rec, req)

	cleared := rec.Result().Cookies()[0]
	if cleared.MaxAge >= 0 {
		t.Errorf("cookie MaxAge = %d, want negative", cleared.MaxAge)
	}
	if _, ok := m.Store().Load(t.Context(), cookie.Value); ok {
		t.Error("session should be gone from the store")
	}
}

func TestFlashesDrainOnce(t *testing.T) {
	s := session.Restore("id", nil, time.Now(), time.Now().Add(time.Hour))
	s.AddFlash("success", "saved")
	s.AddFlash("error", "nope")

	got := s.Flashes()
	if len(got) != 2 || got[0].Message != "saved" || got[1].Kind != "error" {
		t.Fatalf("unexpected flashes: %+v", got)
	}
	if again := s.Flashes(); again != nil {
		t.Errorf("flashes should drain, got %+v", again)
	}
}

func TestExpiredSessionIsNotLoaded(t *testing.T) {
	store := session.NewMemoryStore(0)
	expired := session.Restore("expired", nil, time.Now().Add(-time.Hour), time.Now().Add(-time.Minute))
	if err := store.Save(t.Context(), expired); err != nil {
		t.Fatal(err)
	}
	if !expired.Expired() {
		t.Fatal("session should report itself expired")
	}
	if _, ok := store.Load(t.Context(), "expired"); ok {
		t.Error("expired session should not load")
	}
}

func TestRestoreRoundTripsValues(t *testing.T) {
	created := time.Now().Add(-time.Minute)
	expires := time.Now().Add(time.Hour)
	s := session.Restore("abc", map[string]any{"user": "jane", "count": 3}, created, expires)

	if s.ID() != "abc" {
		t.Errorf("ID = %q", s.ID())
	}
	if !s.CreatedAt().Equal(created) || !s.ExpiresAt().Equal(expires) {
		t.Error("timestamps did not round trip")
	}
	if s.GetString("user") != "jane" || s.GetInt("count") != 3 {
		t.Error("values did not round trip")
	}
	if s.Modified() {
		t.Error("a restored session should start unmodified")
	}

	values := s.Values()
	if len(values) != 2 {
		t.Errorf("Values() = %#v", values)
	}
	values["user"] = "tampered"
	if s.GetString("user") != "jane" {
		t.Error("Values() should return a copy")
	}
}

func TestCSRFTokenIsStableAndValidated(t *testing.T) {
	m := newTestManager()
	rec := httptest.NewRecorder()
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first := m.CSRFToken(r)
		if first == "" {
			t.Fatal("empty csrf token")
		}
		if second := m.CSRFToken(r); second != first {
			t.Error("csrf token should be stable within a session")
		}
		if !m.ValidCSRF(r, first) {
			t.Error("token should validate")
		}
		if m.ValidCSRF(r, "forged") {
			t.Error("forged token should not validate")
		}
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestDeleteByUserID(t *testing.T) {
	store := session.NewMemoryStore(0)
	for _, id := range []string{"a", "b", "c"} {
		s := session.Restore(id, nil, time.Now(), time.Now().Add(time.Hour))
		if id != "c" {
			s.SetUserID("u1")
		}
		if err := store.Save(t.Context(), s); err != nil {
			t.Fatal(err)
		}
	}
	n, err := store.DeleteByUserID(t.Context(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("deleted %d sessions, want 2", n)
	}
	if sessionCount(t, store) != 1 {
		t.Errorf("remaining = %d, want 1", sessionCount(t, store))
	}
}

func TestMemoryStoreSatisfiesManageableStore(t *testing.T) {
	var store session.Store = session.NewMemoryStore(0)
	manageable, ok := store.(session.ManageableStore)
	if !ok {
		t.Fatal("MemoryStore should satisfy ManageableStore")
	}
	if sessionCount(t, manageable) != 0 {
		t.Errorf("Count = %d", sessionCount(t, manageable))
	}
	if len(allSessions(t, manageable)) != 0 {
		t.Error("All should start empty")
	}
}

func TestSafeNextRefusesEverythingOffsite(t *testing.T) {
	hostile := []string{
		"//evil.test/x",
		"/\\evil.test/x",
		"\\/evil.test",
		"https://evil.test/x",
		"http:/evil.test",
		"javascript:alert(1)",
		"/x\r\nSet-Cookie: a=b",
		"/x\n/y",
		"//",
		"evil",
		"",
	}
	for _, next := range hostile {
		if got := view.SafeNext(next, "/safe"); got != "/safe" {
			t.Errorf("SafeNext(%q) = %q, want the fallback", next, got)
		}
	}

	for _, next := range []string{"/", "/dashboard", "/a/b?c=1", "/a#b"} {
		if got := view.SafeNext(next, "/safe"); got != next {
			t.Errorf("SafeNext(%q) = %q, a same-site path should survive", next, got)
		}
	}
}
