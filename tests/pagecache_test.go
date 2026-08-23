package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
)

func newPageCacheApp(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *atomic.Int64) {
	t.Helper()
	base := func(s *settings.Settings) {
		s.PageCache = settings.PageCache{Enabled: true, TTL: time.Minute}
	}
	a := newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)

	var hits atomic.Int64
	a.Get("/page", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "rendered %d", hits.Add(1))
	})
	return a, &hits
}

func TestPageCacheServesTheSecondRequestFromTheCache(t *testing.T) {
	a, renders := newPageCacheApp(t)

	first := httptest.NewRecorder()
	a.Handler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/page", nil))
	if first.Header().Get(middleware.CacheStatusHeader) != middleware.CacheMiss {
		t.Errorf("first response = %q, want MISS", first.Header().Get(middleware.CacheStatusHeader))
	}

	second := httptest.NewRecorder()
	a.Handler().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/page", nil))
	if second.Header().Get(middleware.CacheStatusHeader) != middleware.CacheHit {
		t.Errorf("second response = %q, want HIT", second.Header().Get(middleware.CacheStatusHeader))
	}

	if first.Body.String() != second.Body.String() {
		t.Errorf("a hit should be byte identical: %q then %q", first.Body, second.Body)
	}
	if renders.Load() != 1 {
		t.Errorf("the handler ran %d times, want 1", renders.Load())
	}
}

func TestPageCacheKeepsQueryStringsApart(t *testing.T) {
	a, renders := newPageCacheApp(t)

	for _, target := range []string{"/page?a=1", "/page?a=2", "/page?a=1"} {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	}
	if renders.Load() != 2 {
		t.Errorf("the handler ran %d times, want 2 distinct query strings", renders.Load())
	}
}

func TestPageCacheSkipsPostRequests(t *testing.T) {
	a, _ := newPageCacheApp(t)

	var posts atomic.Int64
	a.Post("/submit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "posted %d", posts.Add(1))
	})

	for range 2 {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/submit", nil))
		if rec.Header().Get(middleware.CacheStatusHeader) != "" {
			t.Errorf("a POST should not be marked by the page cache")
		}
	}
	if posts.Load() != 2 {
		t.Errorf("the handler ran %d times, want 2", posts.Load())
	}
}

func TestPageCacheNeverStoresAResponseThatSetsACookie(t *testing.T) {
	a, renders := newPageCacheApp(t)

	a.Get("/session", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		sess.Set("visits", sess.GetInt("visits")+1)
		fmt.Fprintf(w, "rendered %d", renders.Add(1))
	})

	for range 3 {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/session", nil))
		if rec.Header().Get(middleware.CacheStatusHeader) == middleware.CacheHit {
			t.Fatal("a personalised response must never be served from the page cache")
		}
	}
	if renders.Load() != 3 {
		t.Errorf("the handler ran %d times, want 3", renders.Load())
	}
}

func TestPageCacheHonoursNoStore(t *testing.T) {
	a, renders := newPageCacheApp(t)

	a.Get("/private", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintf(w, "rendered %d", renders.Add(1))
	})

	for range 2 {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/private", nil))
	}
	if renders.Load() != 2 {
		t.Errorf("no-store should prevent caching, handler ran %d times", renders.Load())
	}
}

func TestPageCacheSkipsErrorResponses(t *testing.T) {
	a, renders := newPageCacheApp(t)

	a.Get("/broken", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		renders.Add(1)
	})

	for range 2 {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/broken", nil))
	}
	if renders.Load() != 2 {
		t.Errorf("a 500 must not be cached, handler ran %d times", renders.Load())
	}
}

func TestPageCacheHonoursVary(t *testing.T) {
	a, renders := newPageCacheApp(t)

	a.Get("/varies", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Language")
		fmt.Fprintf(w, "language %s", r.Header.Get("Accept-Language"))
		renders.Add(1)
	})

	request := func(language string) string {
		req := httptest.NewRequest(http.MethodGet, "/varies", nil)
		req.Header.Set("Accept-Language", language)
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		return rec.Body.String()
	}

	if body := request("en"); !strings.Contains(body, "en") {
		t.Fatalf("body = %q", body)
	}
	if body := request("fr"); !strings.Contains(body, "fr") {
		t.Errorf("a different Vary value must not read the other entry: %q", body)
	}
	if body := request("en"); !strings.Contains(body, "en") {
		t.Errorf("the first variant should still be cached: %q", body)
	}
	if renders.Load() != 2 {
		t.Errorf("the handler ran %d times, want 2 variants", renders.Load())
	}
}

func TestPageCacheRespectsMaxAgeFromTheResponse(t *testing.T) {
	a, renders := newPageCacheApp(t)

	a.Get("/brief", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=1")
		fmt.Fprintf(w, "rendered %d", renders.Add(1))
	})

	first := httptest.NewRecorder()
	a.Handler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/brief", nil))
	second := httptest.NewRecorder()
	a.Handler().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/brief", nil))

	if second.Header().Get(middleware.CacheStatusHeader) != middleware.CacheHit {
		t.Errorf("the second request should hit within max-age")
	}
	if renders.Load() != 1 {
		t.Errorf("the handler ran %d times, want 1", renders.Load())
	}
}

func TestPageCacheOnlyCoversTheConfiguredPaths(t *testing.T) {
	a, renders := newPageCacheApp(t, func(s *settings.Settings) {
		s.PageCache = settings.PageCache{
			Enabled: true,
			TTL:     time.Minute,
			Paths:   []string{"/docs"},
			Skip:    []string{"/docs/private"},
		}
	})

	a.Get("/docs/{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "docs %d", renders.Add(1))
	})
	a.Get("/docs/private", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "private %d", renders.Add(1))
	})

	status := func(target string) string {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Header().Get(middleware.CacheStatusHeader)
	}

	status("/docs/")
	if got := status("/docs/"); got != middleware.CacheHit {
		t.Errorf("/docs/ = %q, want HIT", got)
	}

	status("/docs/private")
	if got := status("/docs/private"); got == middleware.CacheHit {
		t.Error("a skipped path must never be cached")
	}
	if got := status("/page"); got != "" {
		t.Errorf("/page is outside Paths, so it should be untouched, got %q", got)
	}
}

func TestPageCacheIsOffByDefault(t *testing.T) {
	a := newTestApp(t)
	var renders atomic.Int64
	a.Get("/page", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "rendered %d", renders.Add(1))
	})

	for range 2 {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/page", nil))
		if rec.Header().Get(middleware.CacheStatusHeader) != "" {
			t.Error("the page cache should not be wired without being asked for")
		}
	}
	if renders.Load() != 2 {
		t.Errorf("the handler ran %d times, want 2", renders.Load())
	}
}

func TestPageCacheSkipsAuthorizedRequests(t *testing.T) {
	a, renders := newPageCacheApp(t)

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/page", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
	}
	if renders.Load() != 2 {
		t.Errorf("an authorized request must bypass the cache, handler ran %d times", renders.Load())
	}
}

func TestPageCacheValidationRejectsAMissingAlias(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.PageCache = settings.PageCache{Enabled: true, TTL: time.Minute, Alias: "nope"}
	})...)
	if err == nil || !strings.Contains(err.Error(), "does not match any entry in Caches") {
		t.Errorf("error = %v, want a complaint about the alias", err)
	}
}

func TestPageCacheValidationRejectsARelativePath(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.PageCache = settings.PageCache{Enabled: true, TTL: time.Minute, Paths: []string{"docs"}}
	})...)
	if err == nil || !strings.Contains(err.Error(), "must start with") {
		t.Errorf("error = %v, want a complaint about the path", err)
	}
}
