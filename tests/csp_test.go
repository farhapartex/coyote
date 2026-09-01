package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/settings"
)

var noncePattern = regexp.MustCompile(`'nonce-([A-Za-z0-9_-]{22})'`)

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
	if strings.ContainsAny(match[1], "+/") {
		t.Errorf("the nonce should avoid characters html/template escapes, got %q", match[1])
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

func TestSecureHeadersCarryTheCrossOriginPolicies(t *testing.T) {
	a := newTestApp(t)
	a.Get("/page", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/page", nil))

	want := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "same-origin",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Permissions-Policy":           "camera=(), microphone=(), geolocation=()",
	}
	for name, value := range want {
		if got := rec.Header().Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestFrameOptionsCanBeRelaxedOrDropped(t *testing.T) {
	same := newTestApp(t, func(s *settings.Settings) { s.Security.FrameOptions = "SAMEORIGIN" })
	same.Get("/page", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	rec := httptest.NewRecorder()
	same.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/page", nil))
	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", got)
	}

	none := newTestApp(t, func(s *settings.Settings) { s.Security.FrameOptions = "" })
	none.Get("/page", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	rec = httptest.NewRecorder()
	none.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/page", nil))
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options = %q, want the header omitted", got)
	}
}

func TestFrameOptionsIsValidated(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Security.FrameOptions = "ALLOW-FROM https://evil.test"
	})...)
	if err == nil || !strings.Contains(err.Error(), "Security.FrameOptions") {
		t.Errorf("error = %v, want a complaint about the frame options", err)
	}
}
