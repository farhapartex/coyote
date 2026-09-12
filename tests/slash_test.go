package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/farhapartex/coyote/core/middleware"
)

func strippedSlash(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	handler := middleware.StripTrailingSlash(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestATrailingSlashRedirectsToThePathWithout(t *testing.T) {
	rec := strippedSlash(t, "/articles/")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/articles" {
		t.Errorf("Location = %q, want /articles", location)
	}
}

func TestAPathWithoutATrailingSlashIsLeftAlone(t *testing.T) {
	rec := strippedSlash(t, "/articles")
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want the handler to have run", rec.Code)
	}
}

func TestTheRootIsLeftAlone(t *testing.T) {
	rec := strippedSlash(t, "/")
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want the handler to have run", rec.Code)
	}
}

func TestTheQuerySurvivesTheRedirect(t *testing.T) {
	rec := strippedSlash(t, "/search/?q=fox&page=2")
	if location := rec.Header().Get("Location"); location != "/search?q=fox&page=2" {
		t.Errorf("Location = %q, want /search?q=fox&page=2", location)
	}
}

func TestASecondLeadingSlashCannotSendTheBrowserOffSite(t *testing.T) {
	for _, target := range []string{"//evil.example/", "///evil.example/", "//evil.example/path/"} {
		rec := strippedSlash(t, target)
		location := rec.Header().Get("Location")
		if len(location) > 1 && location[0] == '/' && location[1] == '/' {
			t.Errorf("%s redirected to %q, which is scheme-relative and leaves the site", target, location)
		}
	}
}

func TestOnlySlashesRedirectToTheRoot(t *testing.T) {
	if location := strippedSlash(t, "//").Header().Get("Location"); location != "/" {
		t.Errorf("Location = %q, want /", location)
	}
}

func TestAnEscapedPathStaysEscapedInTheRedirect(t *testing.T) {
	if location := strippedSlash(t, "/a%20b/").Header().Get("Location"); location != "/a%20b" {
		t.Errorf("Location = %q, want /a%%20b", location)
	}
}
