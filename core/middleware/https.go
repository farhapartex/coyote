package middleware

import (
	"net/http"
	"strings"
)

func RequireHTTPS(trustedProxies int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secureRequest(r, trustedProxies) {
				next.ServeHTTP(w, r)
				return
			}
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})
	}
}

func secureRequest(r *http.Request, trustedProxies int) bool {
	if r.TLS != nil {
		return true
	}
	if trustedProxies < 1 {
		return false
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	return strings.EqualFold(strings.TrimSpace(strings.Split(proto, ",")[0]), "https")
}
