package coyote

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/settings"
)

func TestNewPanicsWithoutSettingsFile(t *testing.T) {
	settings.Reset()
	t.Cleanup(settings.Reset)

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("New should panic when no settings have been configured")
		}
		err, ok := recovered.(error)
		if !ok {
			t.Fatalf("panic value should be an error, got %T", recovered)
		}
		for _, want := range []string{"settings.go", "settings.Configure", "SecretKey"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("panic message should mention %q, got:\n%s", want, err.Error())
			}
		}
	}()
	New()
}

func TestNewReadsConfiguredSettings(t *testing.T) {
	settings.Reset()
	t.Cleanup(settings.Reset)

	settings.Configure(func(s *settings.Settings) {
		s.Debug = true
		s.Server.Port = 4567
		s.Sessions.CookieName = "gate_session"
		s.Templates.FS = testFS()
		s.Templates.Layout = "layouts/base.html"
		s.Auth.PBKDF2Iterations = 1000
	})

	app := New()
	if app.Settings.Addr() != "127.0.0.1:4567" {
		t.Errorf("Addr = %q", app.Settings.Addr())
	}
	if app.Sessions.CookieName() != "gate_session" {
		t.Errorf("cookie name = %q", app.Sessions.CookieName())
	}

	app.Get("/", func(w http.ResponseWriter, r *http.Request) {
		app.Render(w, r, "pages/hello.html", Data{"Name": "gate"})
	})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	if cookies := rec.Result().Cookies(); len(cookies) == 0 || cookies[0].Name != "gate_session" {
		t.Errorf("settings cookie name not applied: %+v", rec.Result().Cookies())
	}
}

func TestSettingsDriveSessionCookie(t *testing.T) {
	app := newTestApp(t, func(s *settings.Settings) {
		s.Sessions.CookieName = "custom"
		s.Sessions.Secure = true
		s.Sessions.SameSite = settings.SameSiteStrict
		s.Sessions.Path = "/app"
		s.Sessions.Domain = "example.com"
	})
	app.Get("/x", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie set")
	}
	c := cookies[0]
	if c.Name != "custom" || !c.Secure || !c.HttpOnly ||
		c.SameSite != http.SameSiteStrictMode || c.Path != "/app" || c.Domain != "example.com" {
		t.Errorf("cookie does not match settings: %+v", c)
	}
}

func TestAllowedHostsRejectsUnlistedHost(t *testing.T) {
	app := newTestApp(t, func(s *settings.Settings) {
		s.Debug = false
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"example.com", ".corp.internal"}
	})
	app.Get("/x", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	cases := []struct {
		host string
		code int
	}{
		{"example.com", http.StatusOK},
		{"example.com:8000", http.StatusOK},
		{"EXAMPLE.COM", http.StatusOK},
		{"corp.internal", http.StatusOK},
		{"api.corp.internal", http.StatusOK},
		{"evil.com", http.StatusBadRequest},
		{"notexample.com", http.StatusBadRequest},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		app.Handler().ServeHTTP(rec, req)
		if rec.Code != c.code {
			t.Errorf("host %q: code %d, want %d", c.host, rec.Code, c.code)
		}
	}
}

func TestAllowedHostsWildcardAndDebug(t *testing.T) {
	wildcard := newTestApp(t, func(s *settings.Settings) {
		s.Debug = false
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"*"}
	})
	wildcard.Get("/x", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Host = "anything.example"
	rec := httptest.NewRecorder()
	wildcard.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("wildcard host: code %d, want 200", rec.Code)
	}

	debugApp := newTestApp(t, func(s *settings.Settings) {
		s.Debug = true
		s.AllowedHosts = nil
	})
	debugApp.Get("/x", func(w http.ResponseWriter, r *http.Request) {})
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Host = "whatever.test"
	rec = httptest.NewRecorder()
	debugApp.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("debug with no AllowedHosts: code %d, want 200", rec.Code)
	}
}

func TestDebugShowsRenderErrorDetail(t *testing.T) {
	app := newTestApp(t)
	app.Get("/missing", func(w http.ResponseWriter, r *http.Request) {
		app.Render(w, r, "pages/does-not-exist.html", nil)
	})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "does-not-exist.html") {
		t.Errorf("debug mode should surface the render error, got %q", rec.Body.String())
	}

	prod := newTestApp(t, func(s *settings.Settings) {
		s.Debug = false
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"*"}
	})
	prod.Get("/missing", func(w http.ResponseWriter, r *http.Request) {
		prod.Render(w, r, "pages/does-not-exist.html", nil)
	})
	rec = httptest.NewRecorder()
	prod.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if strings.Contains(rec.Body.String(), "does-not-exist.html") {
		t.Error("template paths should not leak when Debug is off")
	}
}

func TestPBKDF2IterationsComeFromSettings(t *testing.T) {
	app := newTestApp(t, func(s *settings.Settings) {
		s.Auth.PBKDF2Iterations = 2500
		s.Auth.PasswordMinLength = 10
	})
	if err := app.Auth.ValidatePassword("short1234"); err == nil {
		t.Error("9 characters should fail a 10 character minimum")
	}
	if err := app.Auth.ValidatePassword("longenough12"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
	hash, err := app.Auth.HashPassword("longenough12")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "pbkdf2_sha256$2500$") {
		t.Errorf("hash does not use the configured cost: %q", hash)
	}
}
