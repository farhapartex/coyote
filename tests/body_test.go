package tests

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/settings"
)

func bodyLimitApp(t *testing.T, max int64) (*app.App, *strings.Builder) {
	t.Helper()
	read := &strings.Builder{}
	a := newTestApp(t, withoutCSRF, func(s *settings.Settings) {
		s.Server.MaxBodyBytes = max
	})
	a.Post("/swallow", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "413 too large", http.StatusRequestEntityTooLarge)
			return
		}
		read.Write(body)
		w.Write([]byte("swallowed"))
	})
	return a, read
}

func TestBodyLimitRefusesAnOversizeContentLengthOutright(t *testing.T) {
	a, read := bodyLimitApp(t, 1024)

	req := httptest.NewRequest(http.MethodPost, "/swallow", strings.NewReader(strings.Repeat("x", 4096)))
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	if read.Len() != 0 {
		t.Errorf("the handler read %d bytes; an oversize body should never reach it", read.Len())
	}
}

func TestBodyLimitStillCapsABodyThatDeclaresNoLength(t *testing.T) {
	a, read := bodyLimitApp(t, 1024)

	req := httptest.NewRequest(http.MethodPost, "/swallow", strings.NewReader(strings.Repeat("x", 4096)))
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413; a missing Content-Length must not lift the cap", rec.Code)
	}
	if read.Len() > 1024 {
		t.Errorf("the handler read %d bytes past a 1024 byte cap", read.Len())
	}
}

func TestBodyLimitLetsAnOrdinaryRequestThrough(t *testing.T) {
	a, read := bodyLimitApp(t, 1024)

	req := httptest.NewRequest(http.MethodPost, "/swallow", strings.NewReader("small enough"))
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if read.String() != "small enough" {
		t.Errorf("the handler read %q", read.String())
	}
}

func TestBodyLimitOfZeroLiftsTheCap(t *testing.T) {
	a, read := bodyLimitApp(t, 0)

	req := httptest.NewRequest(http.MethodPost, "/swallow", strings.NewReader(strings.Repeat("x", 4096)))
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if read.Len() != 4096 {
		t.Errorf("the handler read %d bytes, want all 4096", read.Len())
	}
}

func TestBodyLimitDefaultsToThirtyTwoMegabytes(t *testing.T) {
	if got := settings.Default().Server.MaxBodyBytes; got != 32<<20 {
		t.Errorf("MaxBodyBytes = %d, want %d", got, 32<<20)
	}
}

func TestBodyLimitValidationRefusesToStarveUploads(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Server.MaxBodyBytes = 1 << 10
		s.Uploads.Enabled = true
		s.Uploads.Dir = "media"
		s.Uploads.MaxSize = 1 << 20
		s.Uploads.Allowed = []string{"image/png"}
	})...)
	if err == nil || !strings.Contains(err.Error(), "smaller than Uploads.MaxSize") {
		t.Errorf("error = %v, want a complaint about the cap starving uploads", err)
	}
}

func TestBodyLimitValidationRefusesANegativeCap(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Server.MaxBodyBytes = -1
	})...)
	if err == nil || !strings.Contains(err.Error(), "MaxBodyBytes cannot be negative") {
		t.Errorf("error = %v, want a complaint about the negative cap", err)
	}
}
