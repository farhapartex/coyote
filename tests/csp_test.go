package tests

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/settings"
)

var noncePattern = regexp.MustCompile(`'nonce-([A-Za-z0-9+/]{22})'`)

func TestCSPStaticPolicy(t *testing.T) {
	handler := middleware.CSP("default-src 'self'", false)(http.HandlerFunc(noop))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get(middleware.CSPHeader); got != "default-src 'self'" {
		t.Errorf("policy = %q", got)
	}
	if rec.Header().Get(middleware.CSPReportOnlyHeader) != "" {
		t.Error("report-only should not be set")
	}
}

func TestCSPReportOnlyUsesTheOtherHeader(t *testing.T) {
	handler := middleware.CSP("default-src 'self'", true)(http.HandlerFunc(noop))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Header().Get(middleware.CSPHeader) != "" {
		t.Error("the enforcing header must not be set in report-only mode")
	}
	if rec.Header().Get(middleware.CSPReportOnlyHeader) == "" {
		t.Error("report-only header missing")
	}
}

func TestCSPNonceIsPerRequestAndReachable(t *testing.T) {
	var fromContext string
	handler := middleware.CSP("style-src 'self' {nonce}", false)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fromContext = middleware.NonceFrom(r.Context())
		}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	policy := rec.Header().Get(middleware.CSPHeader)
	match := noncePattern.FindStringSubmatch(policy)
	if match == nil {
		t.Fatalf("no nonce in policy %q", policy)
	}
	if match[1] != fromContext {
		t.Errorf("policy nonce %q does not match context nonce %q", match[1], fromContext)
	}
	if strings.Contains(policy, middleware.NoncePlaceholder) {
		t.Error("the placeholder should have been replaced")
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if second.Header().Get(middleware.CSPHeader) == policy {
		t.Error("each request should get a fresh nonce")
	}
}

func TestCSPIsOffByDefault(t *testing.T) {
	a := newTestApp(t)
	a.Get("/x", noop)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Header().Get(middleware.CSPHeader) != "" {
		t.Error("no policy should be sent unless configured")
	}
}

func TestCSPFromSettingsReachesTemplates(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) { s.Security.CSP = settings.DefaultCSP })
	a.Get("/nonced", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/nonce.html", nil)
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nonced", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d: %s", rec.Code, rec.Body.String())
	}

	policy := rec.Header().Get(middleware.CSPHeader)
	match := noncePattern.FindStringSubmatch(policy)
	if match == nil {
		t.Fatalf("no nonce in %q", policy)
	}
	if !strings.Contains(rec.Body.String(), `nonce="`+match[1]+`"`) {
		t.Errorf("the rendered page should carry the same nonce:\n%s", rec.Body.String())
	}
}

func TestDefaultCSPShape(t *testing.T) {
	for _, want := range []string{
		"default-src 'self'", "object-src 'none'", "frame-ancestors 'none'",
		"base-uri 'self'", middleware.NoncePlaceholder,
	} {
		if !strings.Contains(settings.DefaultCSP, want) {
			t.Errorf("DefaultCSP is missing %q", want)
		}
	}
	if strings.Contains(settings.DefaultCSP, "unsafe-inline") {
		t.Error("the default policy should not allow unsafe-inline")
	}
}

func TestCSPReportOnlyWithoutPolicyIsRejected(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) { s.Security.CSPReportOnly = true })...)
	if err == nil {
		t.Fatal("report-only without a policy should be rejected")
	}
	mustContain(t, problemsOf(t, err), "CSPReportOnly")
}
