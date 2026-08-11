package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/router"
	"github.com/farhapartex/coyote/core/session"
)

type Middleware = router.Middleware

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func RequestLogger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			logger.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("took", time.Since(start).Round(time.Microsecond)),
			)
		})
	}
}

func Recoverer(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.Error("panic recovered",
					slog.Any("error", rec),
					slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				http.Error(w, "500 internal server error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func AllowedHosts(hosts []string, debug bool) Middleware {
	allowAny := len(hosts) == 0 && debug
	patterns := make([]string, 0, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "*" {
			allowAny = true
			break
		}
		if host != "" {
			patterns = append(patterns, host)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowAny || hostAllowed(r.Host, patterns) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "400 bad request: host not allowed", http.StatusBadRequest)
		})
	}
}

func hostAllowed(requestHost string, patterns []string) bool {
	host := strings.ToLower(requestHost)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(host, ".")
	for _, pattern := range patterns {
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, ".") && (strings.HasSuffix(host, pattern) || host == pattern[1:]) {
			return true
		}
	}
	return false
}

func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func CSRF(manager *session.Manager) Middleware {
	safe := map[string]bool{
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
		http.MethodTrace:   true,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safe[r.Method] {
				next.ServeHTTP(w, r)
				return
			}
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				if err := r.ParseForm(); err == nil {
					token = r.PostForm.Get("csrf_token")
				}
			}
			if !manager.ValidCSRF(r, token) {
				http.Error(w, "403 CSRF token invalid or missing", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func StripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 1 && strings.HasSuffix(r.URL.Path, "/") {
			target := strings.TrimSuffix(r.URL.Path, "/")
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}
