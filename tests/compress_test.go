package tests

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/settings"
)

func gzipRequest(t *testing.T, handler http.Handler, accept bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if accept {
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func body(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Header().Get("Content-Encoding") != "gzip" {
		return rec.Body.String()
	}
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func html(size int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Repeat("a", size)))
	})
}

func TestCompressLargeResponses(t *testing.T) {
	handler := middleware.Compress(0)(html(4096))
	rec := gzipRequest(t, handler, true)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip, headers: %v", rec.Header())
	}
	if rec.Body.Len() >= 4096 {
		t.Errorf("body was not compressed: %d bytes", rec.Body.Len())
	}
	if got := body(t, rec); got != strings.Repeat("a", 4096) {
		t.Errorf("body did not survive compression: %d bytes", len(got))
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Error("compressed responses must Vary on Accept-Encoding")
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Error("Content-Length must be dropped once the body is compressed")
	}
}

func TestCompressSkippedWithoutAcceptEncoding(t *testing.T) {
	handler := middleware.Compress(0)(html(4096))
	rec := gzipRequest(t, handler, false)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Error("a client that did not ask for gzip must not get it")
	}
	if rec.Body.Len() != 4096 {
		t.Errorf("body = %d bytes, want 4096", rec.Body.Len())
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Error("Vary must be set even when not compressing")
	}
}

func TestCompressSkipsSmallResponses(t *testing.T) {
	handler := middleware.Compress(0)(html(64))
	rec := gzipRequest(t, handler, true)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("a tiny response should not be compressed")
	}
	if rec.Body.String() != strings.Repeat("a", 64) {
		t.Error("small bodies should pass through untouched")
	}
}

func TestCompressSkipsIncompressibleTypes(t *testing.T) {
	for _, contentType := range []string{"image/png", "video/mp4", "application/zip", "application/pdf"} {
		handler := middleware.Compress(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Write([]byte(strings.Repeat("a", 4096)))
		}))
		rec := gzipRequest(t, handler, true)
		if rec.Header().Get("Content-Encoding") == "gzip" {
			t.Errorf("%s should not be gzipped again", contentType)
		}
	}
}

func TestCompressLeavesAlreadyEncodedBodies(t *testing.T) {
	handler := middleware.Compress(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "br")
		w.Write([]byte(strings.Repeat("a", 4096)))
	}))
	rec := gzipRequest(t, handler, true)

	if rec.Header().Get("Content-Encoding") != "br" {
		t.Errorf("Content-Encoding = %q, want br untouched", rec.Header().Get("Content-Encoding"))
	}
}

func TestCompressSkipsEmptyStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
		handler := middleware.Compress(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		rec := gzipRequest(t, handler, true)
		if rec.Header().Get("Content-Encoding") == "gzip" {
			t.Errorf("status %d should not be compressed", status)
		}
	}
}

func TestCompressSupportsFlushing(t *testing.T) {
	handler := middleware.Compress(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(strings.Repeat("a", 2048)))
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("the compressing writer must still support Flusher")
			return
		}
		flusher.Flush()
	}))
	rec := gzipRequest(t, handler, true)
	if rec.Body.Len() == 0 {
		t.Error("flushed data should reach the client")
	}
}

func TestCompressFromSettings(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) { s.Security.Compress = true })
	a.Get("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Repeat("coyote ", 500)))
	})

	req := httptest.NewRequest(http.MethodGet, "/big", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip, got headers %v", rec.Header())
	}
	if got := body(t, rec); !strings.HasPrefix(got, "coyote coyote") {
		t.Errorf("unexpected body: %.40q", got)
	}
}

func TestCompressIsOffByDefault(t *testing.T) {
	a := newTestApp(t)
	a.Get("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("a", 4096)))
	})
	req := httptest.NewRequest(http.MethodGet, "/big", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("compression should be opt-in")
	}
}

func TestCompressLevelValidation(t *testing.T) {
	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Security.Compress = true
		s.Security.CompressLevel = 12
	})...); err == nil {
		t.Error("an out-of-range level should be rejected")
	}
	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Security.CompressLevel = 5
	})...); err == nil {
		t.Error("a level without Compress should be rejected")
	}
	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Security.Compress = true
		s.Security.CompressLevel = 9
	})...); err != nil {
		t.Errorf("a valid level should pass: %v", err)
	}
}
