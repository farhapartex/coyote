package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/view"
)

func TestRoutingAndMethods(t *testing.T) {
	a := newTestApp(t)
	a.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	a.Post("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("posted"))
	})
	a.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("item " + r.PathValue("id")))
	})

	cases := []struct {
		method, path, want string
		code               int
	}{
		{http.MethodGet, "/ping", "pong", http.StatusOK},
		{http.MethodPost, "/ping", "posted", http.StatusOK},
		{http.MethodGet, "/items/42", "item 42", http.StatusOK},
		{http.MethodDelete, "/ping", "", http.StatusMethodNotAllowed},
		{http.MethodGet, "/nope", "", http.StatusNotFound},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s %s: code %d, want %d", c.method, c.path, rec.Code, c.code)
		}
		if c.want != "" && rec.Body.String() != c.want {
			t.Errorf("%s %s: body %q, want %q", c.method, c.path, rec.Body.String(), c.want)
		}
	}
}

func TestGlobalMiddlewareRunsOncePerRequest(t *testing.T) {
	a := newTestApp(t)
	hits := 0
	a.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			next.ServeHTTP(w, r)
		})
	})
	a.Get("/x", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if hits != 1 {
		t.Errorf("middleware ran %d times, want 1", hits)
	}
}

func TestGroupPrefixAndMiddleware(t *testing.T) {
	a := newTestApp(t)
	group := a.Group("/api", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Group", "api")
			next.ServeHTTP(w, r)
		})
	})
	group.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("group route: %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Group") != "api" {
		t.Error("group middleware did not run")
	}

	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusNotFound {
		t.Error("unprefixed path should not resolve")
	}
}

func TestRenderUsesLayoutAndContext(t *testing.T) {
	a := newTestApp(t)
	a.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/hello.html", app.Data{"Name": "Coyote"})
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "<html><body>") {
		t.Error("layout was not applied")
	}
	if !strings.Contains(body, "<h1>Coyote</h1>") {
		t.Error("page data missing")
	}
	if !strings.Contains(body, "<p>/hello</p>") {
		t.Error("context Path missing")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	a := newTestApp(t)
	var token string
	a.Get("/form", func(w http.ResponseWriter, r *http.Request) {
		token = a.Sessions.CSRFToken(r)
		w.Write([]byte(token))
	}, a.CSRF)
	a.Post("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("accepted"))
	}, a.CSRF)

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/form", nil))
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("csrf_token=forged"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("forged token: %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("csrf_token="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "accepted" {
		t.Errorf("valid token: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRecovererReturns500(t *testing.T) {
	a := newTestApp(t)
	a.Get("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code %d, want 500", rec.Code)
	}
}

func TestFlashSurvivesRedirect(t *testing.T) {
	a := newTestApp(t)
	a.Get("/set", func(w http.ResponseWriter, r *http.Request) {
		view.Flash(r, "success", "saved")
		view.Redirect(w, r, "/hello")
	})
	a.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		data := a.Context(r, nil)
		flashes, _ := data["Flashes"].([]session.Flash)
		parts := make([]string, 0, len(flashes))
		for _, f := range flashes {
			parts = append(parts, f.Kind+":"+f.Message)
		}
		w.Write([]byte(strings.Join(parts, ",")))
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("code %d, want 303", rec.Code)
	}
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "success:saved") {
		t.Errorf("flash not delivered, got %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "success:saved") {
		t.Error("flash should only be shown once")
	}
}

func TestRoutesAreRecorded(t *testing.T) {
	a := newTestApp(t)
	a.Get("/a", func(w http.ResponseWriter, r *http.Request) {})
	a.Post("/b", func(w http.ResponseWriter, r *http.Request) {})
	routes := a.Routes()
	if len(routes) != 2 {
		t.Fatalf("recorded %d routes, want 2", len(routes))
	}
	if routes[0].Pattern != "/a" || routes[0].Method != http.MethodGet {
		t.Errorf("unexpected first route: %+v", routes[0])
	}
}

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
	app.New()
}

func TestNewReadsConfiguredSettings(t *testing.T) {
	settings.Reset()
	t.Cleanup(settings.Reset)

	settings.Configure(func(s *settings.Settings) {
		s.Debug = true
		s.Server.Port = 4567
		s.Sessions.CookieName = "gate_session"
		s.Templates.FS = templateFS()
		s.Templates.Layout = "layouts/base.html"
		s.Auth.PBKDF2Iterations = 1000
	})

	a := app.New()
	if a.Settings.Addr() != "127.0.0.1:4567" {
		t.Errorf("Addr = %q", a.Settings.Addr())
	}
	if a.Sessions.CookieName() != "gate_session" {
		t.Errorf("cookie name = %q", a.Sessions.CookieName())
	}

	a.Get("/", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/hello.html", app.Data{"Name": "gate"})
	})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	if cookies := rec.Result().Cookies(); len(cookies) == 0 || cookies[0].Name != "gate_session" {
		t.Errorf("settings cookie name not applied: %+v", rec.Result().Cookies())
	}
}

func TestSettingsDriveSessionCookie(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Sessions.CookieName = "custom"
		s.Sessions.Secure = true
		s.Sessions.SameSite = settings.SameSiteStrict
		s.Sessions.Path = "/app"
		s.Sessions.Domain = "example.com"
	})
	a.Get("/x", func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("who", "jane")
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
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
	a := newTestApp(t, func(s *settings.Settings) {
		s.Debug = false
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"example.com", ".corp.internal"}
	})
	a.Get("/x", func(w http.ResponseWriter, r *http.Request) {
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
		a.Handler().ServeHTTP(rec, req)
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
	a := newTestApp(t)
	a.Get("/missing", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/does-not-exist.html", nil)
	})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
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
	a := newTestApp(t, func(s *settings.Settings) {
		s.Auth.PBKDF2Iterations = 2500
		s.Auth.PasswordMinLength = 10
	})
	if err := a.Auth.ValidatePassword("short1234"); err == nil {
		t.Error("9 characters should fail a 10 character minimum")
	}
	if err := a.Auth.ValidatePassword("longenough12"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
	hash, err := a.Auth.HashPassword("longenough12")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "pbkdf2_sha256$2500$") {
		t.Errorf("hash does not use the configured cost: %q", hash)
	}
}
