package middleware

import (
	"net/http"
	"strconv"
	"time"
)

func HSTS(maxAge time.Duration) Middleware {
	value := "max-age=" + strconv.Itoa(int(maxAge.Seconds())) + "; includeSubDomains"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil || forwardedHTTPS(r) {
				w.Header().Set("Strict-Transport-Security", value)
			}
			next.ServeHTTP(w, r)
		})
	}
}
