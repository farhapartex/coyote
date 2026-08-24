package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/settings"
)

func newCachedLocaleApp(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *atomic.Int64) {
	t.Helper()
	base := func(s *settings.Settings) {
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en", "fr", "ar"},
			FS:        docsLocaleFS(),
		}
		s.PageCache = settings.PageCache{Enabled: true, TTL: time.Minute}
	}
	a := newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)

	var renders atomic.Int64
	a.Get("/notes", func(w http.ResponseWriter, r *http.Request) {
		renders.Add(1)
		fmt.Fprint(w, i18n.T(r.Context(), "Your notes"))
	})
	return a, &renders
}

func fetch(t *testing.T, a *app.App, target string, prepare ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, fn := range prepare {
		fn(req)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func withLanguage(tag string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Accept-Language", tag) }
}

func withLocaleCookie(tag string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "coyote_locale", Value: tag}) }
}

func TestPageCacheKeepsLocalesApartByHeader(t *testing.T) {
	a, renders := newCachedLocaleApp(t)

	first := fetch(t, a, "/notes", withLanguage("fr"))
	if body := first.Body.String(); body != "Vos notes" {
		t.Fatalf("French body = %q", body)
	}

	second := fetch(t, a, "/notes", withLanguage("en"))
	if body := second.Body.String(); body != "Your notes" {
		t.Errorf("English body = %q; the French page was served from the cache", body)
	}
	if second.Header().Get(middleware.CacheStatusHeader) == middleware.CacheHit {
		t.Error("a different locale must not hit the other locale's entry")
	}

	repeat := fetch(t, a, "/notes", withLanguage("fr"))
	if repeat.Header().Get(middleware.CacheStatusHeader) != middleware.CacheHit {
		t.Errorf("the same locale should hit, got %q", repeat.Header().Get(middleware.CacheStatusHeader))
	}
	if body := repeat.Body.String(); body != "Vos notes" {
		t.Errorf("cached French body = %q", body)
	}
	if renders.Load() != 2 {
		t.Errorf("the handler ran %d times, want one per locale", renders.Load())
	}
}

func TestPageCacheKeepsLocalesApartByCookie(t *testing.T) {
	a, renders := newCachedLocaleApp(t)

	if body := fetch(t, a, "/notes", withLocaleCookie("fr")).Body.String(); body != "Vos notes" {
		t.Fatalf("French body = %q", body)
	}
	if body := fetch(t, a, "/notes", withLocaleCookie("ar")).Body.String(); body != "ملاحظاتك" {
		t.Errorf("Arabic body = %q; a cookie-chosen locale shared an entry", body)
	}
	if body := fetch(t, a, "/notes").Body.String(); body != "Your notes" {
		t.Errorf("default body = %q", body)
	}
	if renders.Load() != 3 {
		t.Errorf("the handler ran %d times, want one per locale", renders.Load())
	}
}

func TestPageCacheVariesOnAcceptLanguage(t *testing.T) {
	a, _ := newCachedLocaleApp(t)

	rec := fetch(t, a, "/notes", withLanguage("fr"))
	if vary := rec.Header().Get("Vary"); !strings.Contains(vary, "Accept-Language") {
		t.Errorf("Vary = %q", vary)
	}
}

func TestFragmentCacheKeepsLocalesApart(t *testing.T) {
	templates := fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{Data: []byte(
			`{{define "base.html"}}{{block "content" .}}{{end}}{{end}}`)},
		"partials/heading.html": &fstest.MapFile{Data: []byte(
			`{{define "heading.html"}}<h1>{{.Locale.T "Your notes"}}</h1>{{end}}`)},
		"pages/notes.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}{{fragment "heading.html" 300 "shared" .}}{{end}}`)},
	}

	a := newTestApp(t, func(s *settings.Settings) {
		s.Templates.FS = templates
		s.Templates.Layout = "layouts/base.html"
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en", "fr", "ar"},
			FS:        docsLocaleFS(),
		}
	})
	a.Get("/notes", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/notes.html", app.Data{})
	})

	if body := fetch(t, a, "/notes", withLanguage("fr")).Body.String(); !strings.Contains(body, "Vos notes") {
		t.Fatalf("French fragment = %q", body)
	}

	english := fetch(t, a, "/notes", withLanguage("en")).Body.String()
	if strings.Contains(english, "Vos notes") {
		t.Errorf("the French fragment was served to an English request: %q", english)
	}
	if !strings.Contains(english, "Your notes") {
		t.Errorf("English fragment = %q", english)
	}

	arabic := fetch(t, a, "/notes", withLanguage("ar")).Body.String()
	if !strings.Contains(arabic, "ملاحظاتك") {
		t.Errorf("Arabic fragment = %q", arabic)
	}

	repeat := fetch(t, a, "/notes", withLanguage("fr")).Body.String()
	if !strings.Contains(repeat, "Vos notes") {
		t.Errorf("the French fragment should still be cached: %q", repeat)
	}
}

func TestFragmentCacheKeyIsUnaffectedWithoutALocale(t *testing.T) {
	templates := fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{Data: []byte(
			`{{define "base.html"}}{{block "content" .}}{{end}}{{end}}`)},
		"partials/counter.html": &fstest.MapFile{Data: []byte(
			`{{define "counter.html"}}<span>{{.Count}}</span>{{end}}`)},
		"pages/counter.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}{{fragment "counter.html" 300 "shared" .}}{{end}}`)},
	}

	a := newTestApp(t, func(s *settings.Settings) {
		s.Templates.FS = templates
		s.Templates.Layout = "layouts/base.html"
	})
	a.Get("/one", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/counter.html", app.Data{"Count": 1})
	})
	a.Get("/two", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/counter.html", app.Data{"Count": 2})
	})

	if body := fetch(t, a, "/one").Body.String(); !strings.Contains(body, "<span>1</span>") {
		t.Fatalf("body = %q", body)
	}
	if body := fetch(t, a, "/two").Body.String(); !strings.Contains(body, "<span>1</span>") {
		t.Errorf("a monolingual project should still share one entry, got %q", body)
	}
}
