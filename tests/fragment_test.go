package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/settings"
)

func fragmentFS() fstest.MapFS {
	files := templateFS()
	files["partials/counter.html"] = &fstest.MapFile{Data: []byte(
		`{{define "counter.html"}}<span>{{.Count}}</span>{{end}}`)}
	files["partials/secret.html"] = &fstest.MapFile{Data: []byte(
		`{{define "secret.html"}}<input value="{{.CSRFToken}}">{{end}}`)}
	files["pages/fragment.html"] = &fstest.MapFile{Data: []byte(
		`{{define "content"}}{{fragment "counter.html" 300 .Who .}}{{end}}`)}
	files["pages/leaky.html"] = &fstest.MapFile{Data: []byte(
		`{{define "content"}}{{fragment "secret.html" 300 "shared" .}}{{end}}`)}
	files["pages/missing.html"] = &fstest.MapFile{Data: []byte(
		`{{define "content"}}{{fragment "nope.html" 300 "x" .}}{{end}}`)}
	return files
}

func newFragmentApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	base := func(s *settings.Settings) { s.Templates.FS = fragmentFS() }
	return newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)
}

func renderPage(t *testing.T, a *app.App, page string, data app.Data) string {
	t.Helper()
	a.Get("/"+page, func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/"+page+".html", data)
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+page, nil))
	return rec.Body.String()
}

func TestFragmentIsCachedBetweenRenders(t *testing.T) {
	a := newFragmentApp(t)

	first := renderPage(t, a, "fragment", app.Data{"Who": "ann", "Count": 1})
	if !strings.Contains(first, "<span>1</span>") {
		t.Fatalf("first render = %q", first)
	}

	rec := httptest.NewRecorder()
	a.Get("/second", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/fragment.html", app.Data{"Who": "ann", "Count": 2})
	})
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/second", nil))

	if !strings.Contains(rec.Body.String(), "<span>1</span>") {
		t.Errorf("the cached fragment should be reused, got %q", rec.Body.String())
	}
}

func TestFragmentKeyPartSeparatesCallers(t *testing.T) {
	a := newFragmentApp(t)

	a.Get("/one", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/fragment.html", app.Data{"Who": "ann", "Count": 1})
	})
	a.Get("/two", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/fragment.html", app.Data{"Who": "bob", "Count": 2})
	})

	first := httptest.NewRecorder()
	a.Handler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/one", nil))
	second := httptest.NewRecorder()
	a.Handler().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/two", nil))

	if !strings.Contains(first.Body.String(), "<span>1</span>") {
		t.Errorf("first = %q", first.Body.String())
	}
	if !strings.Contains(second.Body.String(), "<span>2</span>") {
		t.Errorf("a different key part must not read the other entry: %q", second.Body.String())
	}
}

func TestFragmentIsNotCachedWhenItHoldsTheCSRFToken(t *testing.T) {
	a := newFragmentApp(t)

	a.Get("/leaky", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/leaky.html", app.Data{})
	})

	first := httptest.NewRecorder()
	a.Handler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/leaky", nil))

	second := httptest.NewRecorder()
	a.Handler().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/leaky", nil))

	firstToken := valueOf(t, first.Body.String())
	secondToken := valueOf(t, second.Body.String())

	if firstToken == "" || secondToken == "" {
		t.Fatalf("expected a token in both renders, got %q and %q", firstToken, secondToken)
	}
	if firstToken == secondToken {
		t.Error("a fragment holding the CSRF token must never be served from the cache")
	}
}

func TestFragmentWithoutACacheStillRenders(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Templates.FS = fragmentFS()
		s.Caches = []settings.Cache{{Backend: settings.CacheInMemory}}
	})

	body := renderPage(t, a, "fragment", app.Data{"Who": "ann", "Count": 7})
	if !strings.Contains(body, "<span>7</span>") {
		t.Errorf("body = %q", body)
	}
}

func TestFragmentReportsAnUnknownName(t *testing.T) {
	a := newFragmentApp(t)

	a.Get("/missing", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/missing.html", app.Data{})
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for an unknown fragment", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no fragment named") {
		t.Errorf("body should explain the problem under Debug: %q", rec.Body.String())
	}
}

func valueOf(t *testing.T, body string) string {
	t.Helper()
	const marker = `value="`
	start := strings.Index(body, marker)
	if start < 0 {
		return ""
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}
