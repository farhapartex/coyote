package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(Options{
		Store:    NewMemoryStore(0),
		Lifetime: time.Hour,
		HTTPOnly: true,
	})
}

func TestSessionPersistsAcrossRequests(t *testing.T) {
	m := newTestManager()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := FromRequest(r)
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
			s := FromRequest(r)
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
		FromRequest(r).Set("user", "jane")
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
		kept = FromRequest(r).GetString("user")
	})).ServeHTTP(rec, req)

	second := rec.Result().Cookies()[0]
	if second.Value == first.Value {
		t.Error("session id was not rotated")
	}
	if kept != "jane" {
		t.Errorf("values lost on renew: %q", kept)
	}
	if _, ok := store.Load(first.Value); ok {
		t.Error("old session should be evicted from the store")
	}
	if _, ok := store.Load(second.Value); !ok {
		t.Error("new session should be in the store")
	}
}

func TestDestroyClearsCookieAndStore(t *testing.T) {
	m := newTestManager()

	rec := httptest.NewRecorder()
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromRequest(r).Set("user", "jane")
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
	if _, ok := m.Store().Load(cookie.Value); ok {
		t.Error("session should be gone from the store")
	}
}

func TestFlashesDrainOnce(t *testing.T) {
	s := newSession("id", time.Hour)
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
	store := NewMemoryStore(0)
	s := newSession("expired", -time.Minute)
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Load("expired"); ok {
		t.Error("expired session should not load")
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
	store := NewMemoryStore(0)
	for _, id := range []string{"a", "b", "c"} {
		s := newSession(id, time.Hour)
		if id != "c" {
			s.SetUserID("u1")
		}
		if err := store.Save(s); err != nil {
			t.Fatal(err)
		}
	}
	if n := store.DeleteByUserID("u1"); n != 2 {
		t.Errorf("deleted %d sessions, want 2", n)
	}
	if store.Count() != 1 {
		t.Errorf("remaining = %d, want 1", store.Count())
	}
}
