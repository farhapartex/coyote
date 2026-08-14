package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/settings"
)

func corsApp(t *testing.T, mutate func(*settings.CORS)) *client {
	t.Helper()
	a := newTestApp(t, func(s *settings.Settings) {
		policy := settings.CORS{Origins: []string{"https://app.example.com"}}
		mutate(&policy)
		s.Security.CORS = policy
	})
	a.Get("/api", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	a.Post("/api", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("posted")) })
	return newClient(t, a.Handler())
}

func request(t *testing.T, c *client, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return rec
}

func TestCORSAllowsAListedOrigin(t *testing.T) {
	c := corsApp(t, func(*settings.CORS) {})
	rec := request(t, c, http.MethodGet, "/api", map[string]string{"Origin": "https://app.example.com"})

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Error("responses that depend on Origin must Vary on it")
	}
}

func TestCORSIgnoresRequestsWithoutOrigin(t *testing.T) {
	c := corsApp(t, func(*settings.CORS) {})
	rec := request(t, c, http.MethodGet, "/api", nil)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("a same-origin request should not get CORS headers")
	}
	if rec.Body.String() != "ok" {
		t.Error("the handler should still run")
	}
}

func TestCORSDeniesAnUnlistedOrigin(t *testing.T) {
	c := corsApp(t, func(*settings.CORS) {})

	rec := request(t, c, http.MethodGet, "/api", map[string]string{"Origin": "https://evil.example"})
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("an unlisted origin must not be allowed")
	}
	if rec.Body.String() != "ok" {
		t.Error("a simple request still runs; the browser enforces the block")
	}

	preflight := request(t, c, http.MethodOptions, "/api", map[string]string{
		"Origin":                        "https://evil.example",
		"Access-Control-Request-Method": "POST",
	})
	if preflight.Code != http.StatusForbidden {
		t.Errorf("an unlisted preflight should be refused, got %d", preflight.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	c := corsApp(t, func(policy *settings.CORS) {
		policy.MaxAge = 10 * time.Minute
		policy.ExposeHeaders = []string{"X-Total-Count"}
	})

	rec := request(t, c, http.MethodOptions, "/api", map[string]string{
		"Origin":                         "https://app.example.com",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "Content-Type",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want 204", rec.Code)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":   "https://app.example.com",
		"Access-Control-Max-Age":        "600",
		"Access-Control-Expose-Headers": "X-Total-Count",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("Allow-Methods = %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type") {
		t.Errorf("Allow-Headers = %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORSWildcardWithoutCredentials(t *testing.T) {
	c := corsApp(t, func(policy *settings.CORS) { policy.Origins = []string{"*"} })
	rec := request(t, c, http.MethodGet, "/api", map[string]string{"Origin": "https://anywhere.example"})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
}

func TestCORSCredentialsEchoTheOrigin(t *testing.T) {
	c := corsApp(t, func(policy *settings.CORS) { policy.AllowCredentials = true })
	rec := request(t, c, http.MethodGet, "/api", map[string]string{"Origin": "https://app.example.com"})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("credentialed responses must echo the origin, got %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Allow-Credentials missing")
	}
}

func TestCORSIsOffByDefault(t *testing.T) {
	a := newTestApp(t)
	a.Get("/api", noop)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "https://anywhere.example")
	a.Handler().ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS should be off until origins are listed")
	}
}

func TestCORSValidation(t *testing.T) {
	cases := map[string]settings.CORS{
		"cannot combine the \"*\" origin": {Origins: []string{"*"}, AllowCredentials: true},
		"alongside other origins":         {Origins: []string{"*", "https://a.example"}},
		"needs a scheme":                  {Origins: []string{"app.example.com"}},
		"must not end with a slash":       {Origins: []string{"https://app.example.com/"}},
		"empty origin":                    {Origins: []string{"  "}},
		"MaxAge cannot be negative":       {Origins: []string{"https://a.example"}, MaxAge: -time.Second},
	}
	for want, policy := range cases {
		_, err := settings.New(prodSettings(func(s *settings.Settings) { s.Security.CORS = policy })...)
		if err == nil {
			t.Errorf("expected an error mentioning %q", want)
			continue
		}
		mustContain(t, problemsOf(t, err), want)
	}
}
