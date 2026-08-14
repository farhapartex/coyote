package tests

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/settings"
)

var shortID = regexp.MustCompile(`^[0-9a-f]{12}$`)

func TestRequestIDIsGeneratedAndExposed(t *testing.T) {
	var seen string
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = middleware.RequestIDFrom(r.Context())
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	header := rec.Header().Get(middleware.RequestIDHeader)
	if !shortID.MatchString(header) {
		t.Errorf("response header = %q, want 12 hex characters", header)
	}
	if seen != header {
		t.Errorf("context id %q does not match header %q", seen, header)
	}
}

func TestRequestIDsAreUnique(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		value := rec.Header().Get(middleware.RequestIDHeader)
		if seen[value] {
			t.Fatalf("duplicate request id %q", value)
		}
		seen[value] = true
	}
}

func TestUntrustedRequestIDIsIgnored(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, "client-supplied")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(middleware.RequestIDHeader); got == "client-supplied" {
		t.Error("an inbound id must not be trusted by default")
	}
}

func TestTrustedRequestIDHonoursSaneValues(t *testing.T) {
	handler := middleware.TrustedRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, "edge-abc123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get(middleware.RequestIDHeader); got != "edge-abc123" {
		t.Errorf("a trusted id should be kept, got %q", got)
	}

	for _, junk := range []string{"has spaces", strings.Repeat("x", 65), "new\nline", ""} {
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(middleware.RequestIDHeader, junk)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get(middleware.RequestIDHeader); got == junk {
			t.Errorf("a malformed id %q should be replaced", junk)
		}
	}
}

func TestRequestIDAppearsInTheAccessLog(t *testing.T) {
	buffer := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buffer, nil))

	a := newTestApp(t, func(s *settings.Settings) { s.Logging.Logger = logger })
	a.Get("/logged", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logged", nil))

	header := rec.Header().Get(middleware.RequestIDHeader)
	if header == "" {
		t.Fatal("the app should set a request id by default")
	}
	if !strings.Contains(buffer.String(), "request_id="+header) {
		t.Errorf("the access log should carry request_id=%s:\n%s", header, buffer.String())
	}
}

func TestTrustRequestIDSetting(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) { s.Security.TrustRequestID = true })
	a.Get("/x", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(middleware.RequestIDHeader, "from-the-edge")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get(middleware.RequestIDHeader); got != "from-the-edge" {
		t.Errorf("TrustRequestID should honour the inbound header, got %q", got)
	}
}
