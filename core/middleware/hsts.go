package middleware

import (
	"net/http"
	"strconv"
	"time"
)

func HSTS(maxAge time.Duration, trustedProxies int) Middleware {
	value := "max-age=" + strconv.Itoa(int(maxAge.Seconds())) + "; includeSubDomains"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secureRequest(r, trustedProxies) {
				w.Header().Set("Strict-Transport-Security", value)
			}
			next.ServeHTTP(w, r)
		})
	}
}
