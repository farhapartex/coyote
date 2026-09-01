package middleware

import (
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/session"
)

const multipartMemory = 1 << 20

var safeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

func formToken(r *http.Request) string {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(multipartMemory); err != nil {
			return ""
		}
		return r.PostForm.Get("csrf_token")
	}
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.PostForm.Get("csrf_token")
}

func CSRF(manager *session.Manager, exempt []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safeMethods[r.Method] || matchesAnyPrefix(r.URL.Path, exempt) {
				next.ServeHTTP(w, r)
				return
			}
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				token = formToken(r)
			}
			if !manager.ValidCSRF(r, token) {
				http.Error(w, "403 CSRF token invalid or missing", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
