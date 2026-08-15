package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
)

func cookieApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	base := func(s *settings.Settings) { s.Sessions.Backend = settings.SessionsInCookie }
	return app.NewFrom(devSettings(t, append([]func(*settings.Settings){base}, fns...)...))
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "coyote_session" {
			return c
		}
	}
	t.Fatal("no session cookie was set")
	return nil
}

func TestCookieBackendKeepsNothingOnTheServer(t *testing.T) {
	a := cookieApp(t)

	if a.SessionStore() != nil {
		t.Error("a cookie session must not allocate a server-side store")
	}
	if !a.Sessions.Stateless() {
		t.Error("the manager should report itself stateless")
	}
	if _, ok := a.ManageableSessions(); ok {
		t.Error("cookie sessions cannot be listed or revoked from the admin")
	}
	for _, m := range a.Models() {
		if _, ok := m.Entity.(session.Record); ok {
			t.Error("the cookie backend should not register a sessions table")
		}
	}
}

func TestCookieSessionSurvivesARestart(t *testing.T) {
	a := cookieApp(t)
	a.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		sess.Set("visits", 1)
		sess.Set("who", "jane")
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookie := sessionCookie(t, rec)

	restarted := app.NewFrom(a.Settings)
	restarted.Get("/read", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		if _, isInt := sess.Get("visits").(int); !isInt {
			t.Errorf("visits should stay an int, got %T", sess.Get("visits"))
		}
		w.Write([]byte(sess.GetString("who")))
	})

	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	restarted.Handler().ServeHTTP(rec, req)

	if rec.Body.String() != "jane" {
		t.Errorf("the session should travel in the cookie, got %q", rec.Body.String())
	}
}

func TestCookieSessionIsOpaqueAndAuthenticated(t *testing.T) {
	a := cookieApp(t)
	a.Get("/set", func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("who", "jane")
	})
	a.Get("/read", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(session.FromRequest(r).GetString("who")))
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set", nil))
	cookie := sessionCookie(t, rec)

	if strings.Contains(cookie.Value, "jane") {
		t.Error("the cookie should be encrypted, not merely signed")
	}

	tampered := &http.Cookie{Name: cookie.Name, Value: flipLast(cookie.Value)}
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.AddCookie(tampered)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Body.String() != "" {
		t.Errorf("a tampered cookie must be discarded, got %q", rec.Body.String())
	}
}

func TestCookieSessionIsRejectedByAnotherSecretKey(t *testing.T) {
	a := cookieApp(t)
	a.Get("/set", func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("who", "jane")
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set", nil))
	cookie := sessionCookie(t, rec)

	other := cookieApp(t, func(s *settings.Settings) { s.SecretKey = strings.Repeat("z", 48) })
	other.Get("/read", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(session.FromRequest(r).GetString("who")))
	})

	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	other.Handler().ServeHTTP(rec, req)

	if rec.Body.String() != "" {
		t.Error("a cookie sealed with a different key must not open")
	}
}

func TestCookieSessionExpiryTravelsInsideTheSeal(t *testing.T) {
	a := cookieApp(t, func(s *settings.Settings) { s.Sessions.Lifetime = 40 * time.Millisecond })
	a.Get("/set", func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("who", "jane")
	})
	a.Get("/read", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(session.FromRequest(r).GetString("who")))
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set", nil))
	cookie := sessionCookie(t, rec)

	time.Sleep(60 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Body.String() != "" {
		t.Error("an expired seal must be refused even when the browser still sends it")
	}
}

func TestCookieSessionRefusesToOverflowTheCookie(t *testing.T) {
	a := cookieApp(t)
	a.Get("/fat", func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("blob", strings.Repeat("x", 8000))
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fat", nil))

	for _, c := range rec.Result().Cookies() {
		if c.Name == "coyote_session" {
			t.Fatalf("an oversized session must not be written, got %d bytes", len(c.Value))
		}
	}
}

func TestUnmodifiedRequestWritesNoCookie(t *testing.T) {
	a := cookieApp(t)
	a.Get("/idle", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/idle", nil))

	for _, c := range rec.Result().Cookies() {
		if c.Name == "coyote_session" {
			t.Error("a request that never touches the session should not reseal it")
		}
	}
}

func TestSealerRoundTripAndRejection(t *testing.T) {
	sealer, err := session.NewSealer(testSecretKey)
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := sealer.Seal([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := sealer.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != "payload" {
		t.Errorf("round trip = %q", opened)
	}

	again, err := sealer.Seal([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if again == sealed {
		t.Error("each seal should use a fresh nonce")
	}

	for _, bad := range []string{"", "not base64 $$", flipLast(sealed), sealed[:8]} {
		if _, err := sealer.Open(bad); err == nil {
			t.Errorf("Open(%q) should have failed", bad)
		}
	}

	if _, err := sealer.Seal(make([]byte, 6000)); err == nil {
		t.Error("an oversized payload should be refused")
	}
	if _, err := session.NewSealer(""); err == nil {
		t.Error("an empty secret should be refused")
	}
}

func TestCookieBackendValidation(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.Backend = settings.SessionsInCookie
		s.SecretKey = ""
	})...)
	if err == nil {
		t.Fatal("cookie sessions without a SecretKey should be rejected")
	}
	mustContain(t, problemsOf(t, err), "SecretKey is empty")

	_, err = settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.Backend = settings.SessionsInCookie
		s.Sessions.Store = session.NewMemoryStore(0)
	})...)
	if err == nil {
		t.Fatal("a cookie backend with an explicit Store should be rejected")
	}
	mustContain(t, problemsOf(t, err), "choose one")

	_, err = settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.Backend = "banana"
	})...)
	if err == nil {
		t.Fatal("an unknown backend should be rejected")
	}
	mustContain(t, problemsOf(t, err), "\"cookie\"")
}

func flipLast(value string) string {
	if value == "" {
		return "x"
	}
	if value[0] == 'A' {
		return "B" + value[1:]
	}
	return "A" + value[1:]
}
