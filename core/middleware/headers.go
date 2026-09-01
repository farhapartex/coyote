package middleware

import (
	"net/http"

	"github.com/farhapartex/coyote/core/settings"
)

func SecureHeaders(policy settings.Security) Middleware {
	frame := policy.FrameOptions
	permissions := policy.PermissionsPolicy

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			if frame != "" {
				h.Set("X-Frame-Options", frame)
			}
			if permissions != "" {
				h.Set("Permissions-Policy", permissions)
			}
			next.ServeHTTP(w, r)
		})
	}
}
