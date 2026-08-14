package middleware

import (
	"net/http"
	"strings"
)

func RequireHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil || forwardedHTTPS(r) {
			next.ServeHTTP(w, r)
			return
		}
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

func forwardedHTTPS(r *http.Request) bool {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.EqualFold(strings.TrimSpace(strings.Split(proto, ",")[0]), "https")
	}
	return false
}
