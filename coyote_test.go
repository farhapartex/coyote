package coyote

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/session"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{Data: []byte(
			`{{define "base.html"}}<html><body>{{block "content" .}}{{end}}</body></html>{{end}}`)},
		"pages/hello.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<h1>{{.Name}}</h1><p>{{.Path}}</p>{{end}}`)},
	}
}

func newTestApp() *App {
	return New(Config{Templates: testFS(), Layout: "layouts/base.html"})
}

func TestRoutingAndMethods(t *testing.T) {
	app := newTestApp()
	app.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	app.Post("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("posted"))
	})
	app.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
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
		app.Handler().ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s %s: code %d, want %d", c.method, c.path, rec.Code, c.code)
		}
		if c.want != "" && rec.Body.String() != c.want {
			t.Errorf("%s %s: body %q, want %q", c.method, c.path, rec.Body.String(), c.want)
		}
	}
}

func TestGlobalMiddlewareRunsOncePerRequest(t *testing.T) {
	app := newTestApp()
	hits := 0
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			next.ServeHTTP(w, r)
		})
	})
	app.Get("/x", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if hits != 1 {
		t.Errorf("middleware ran %d times, want 1", hits)
	}
}

func TestGroupPrefixAndMiddleware(t *testing.T) {
	app := newTestApp()
	group := app.Group("/api", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Group", "api")
			next.ServeHTTP(w, r)
		})
	})
	group.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("group route: %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Group") != "api" {
		t.Error("group middleware did not run")
	}

	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusNotFound {
		t.Error("unprefixed path should not resolve")
	}
}

func TestRenderUsesLayoutAndContext(t *testing.T) {
	app := newTestApp()
	app.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		app.Render(w, r, "pages/hello.html", Data{"Name": "Coyote"})
	})

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello", nil))
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
	app := newTestApp()
	var token string
	app.Get("/form", func(w http.ResponseWriter, r *http.Request) {
		token = app.Sessions.CSRFToken(r)
		w.Write([]byte(token))
	}, app.CSRF)
	app.Post("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("accepted"))
	}, app.CSRF)

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/form", nil))
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("csrf_token=forged"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("forged token: %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("csrf_token="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "accepted" {
		t.Errorf("valid token: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRecovererReturns500(t *testing.T) {
	app := newTestApp()
	app.Get("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code %d, want 500", rec.Code)
	}
}

func TestFlashSurvivesRedirect(t *testing.T) {
	app := newTestApp()
	app.Get("/set", func(w http.ResponseWriter, r *http.Request) {
		Flash(r, "success", "saved")
		Redirect(w, r, "/hello")
	})
	app.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		data := app.Context(r, nil)
		flashes, _ := data["Flashes"].([]session.Flash)
		parts := make([]string, 0, len(flashes))
		for _, f := range flashes {
			parts = append(parts, f.Kind+":"+f.Message)
		}
		w.Write([]byte(strings.Join(parts, ",")))
	})

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("code %d, want 303", rec.Code)
	}
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "success:saved") {
		t.Errorf("flash not delivered, got %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "success:saved") {
		t.Error("flash should only be shown once")
	}
}

func TestRoutesAreRecorded(t *testing.T) {
	app := newTestApp()
	app.Get("/a", func(w http.ResponseWriter, r *http.Request) {})
	app.Post("/b", func(w http.ResponseWriter, r *http.Request) {})
	routes := app.Routes()
	if len(routes) != 2 {
		t.Fatalf("recorded %d routes, want 2", len(routes))
	}
	if routes[0].Pattern != "/a" || routes[0].Method != http.MethodGet {
		t.Errorf("unexpected first route: %+v", routes[0])
	}
}
