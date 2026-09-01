package middleware

import (
	"bufio"
	"compress/gzip"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const MinCompressSize = 1024

var incompressible = []string{
	"image/", "video/", "audio/", "font/woff",
	"application/zip", "application/gzip", "application/x-gzip",
	"application/pdf", "application/octet-stream",
}

func Compress(level int) Middleware {
	if level < gzip.BestSpeed || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	pool := &sync.Pool{New: func() any {
		writer, _ := gzip.NewWriterLevel(nil, level)
		return writer
	}}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Accept-Encoding")
			if !acceptsGzip(r) {
				next.ServeHTTP(w, r)
				return
			}

			writer := pool.Get().(*gzip.Writer)
			defer pool.Put(writer)

			compressed := &gzipWriter{ResponseWriter: w, gzip: writer}
			defer compressed.Close()
			next.ServeHTTP(compressed, r)
		})
	}
}

func acceptsGzip(r *http.Request) bool {
	for _, encoding := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, _, _ := strings.Cut(strings.TrimSpace(encoding), ";")
		if strings.EqualFold(name, "gzip") {
			return true
		}
	}
	return false
}

type gzipWriter struct {
	http.ResponseWriter
	gzip    *gzip.Writer
	active  bool
	decided bool
	status  int
}

func (g *gzipWriter) WriteHeader(status int) {
	if g.decided {
		return
	}
	g.decided = true
	g.status = status

	if g.shouldCompress(status) {
		g.active = true
		g.Header().Del("Content-Length")
		g.Header().Set("Content-Encoding", "gzip")
		g.gzip.Reset(g.ResponseWriter)
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.decided {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		if g.Header().Get("Content-Length") == "" && len(b) < MinCompressSize {
			g.Header().Set("Content-Length", strconv.Itoa(len(b)))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.active {
		return g.gzip.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipWriter) Flush() {
	if g.active {
		g.gzip.Flush()
	}
	if flusher, ok := g.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func (g *gzipWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(g.ResponseWriter).Hijack()
}

func (g *gzipWriter) Close() {
	if g.active {
		g.gzip.Close()
	}
}

func (g *gzipWriter) shouldCompress(status int) bool {
	if status == http.StatusNoContent || status == http.StatusNotModified {
		return false
	}
	header := g.Header()
	if header.Get("Content-Encoding") != "" {
		return false
	}
	if size, err := strconv.Atoi(header.Get("Content-Length")); err == nil && size < MinCompressSize {
		return false
	}
	contentType := strings.ToLower(header.Get("Content-Type"))
	for _, prefix := range incompressible {
		if strings.HasPrefix(contentType, prefix) {
			return false
		}
	}
	return true
}
