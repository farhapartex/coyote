package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/farhapartex/coyote/core/settings"
)

func CORS(policy settings.CORS) Middleware {
	allowed := make(map[string]bool, len(policy.Origins))
	for _, origin := range policy.Origins {
		allowed[strings.ToLower(strings.TrimSpace(origin))] = true
	}
	any := policy.AllowsAnyOrigin()
	methods := strings.Join(policy.MethodList(), ", ")
	headers := strings.Join(policy.HeaderList(), ", ")
	expose := strings.Join(policy.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(int(policy.MaxAge.Seconds()))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			header := w.Header()
			header.Add("Vary", "Origin")
			if !any && !allowed[strings.ToLower(origin)] {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if any && !policy.AllowCredentials {
				header.Set("Access-Control-Allow-Origin", "*")
			} else {
				header.Set("Access-Control-Allow-Origin", origin)
			}
			if policy.AllowCredentials {
				header.Set("Access-Control-Allow-Credentials", "true")
			}
			if expose != "" {
				header.Set("Access-Control-Expose-Headers", expose)
			}

			if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
				next.ServeHTTP(w, r)
				return
			}

			header.Add("Vary", "Access-Control-Request-Method")
			header.Add("Vary", "Access-Control-Request-Headers")
			header.Set("Access-Control-Allow-Methods", methods)
			header.Set("Access-Control-Allow-Headers", headers)
			if policy.MaxAge > 0 {
				header.Set("Access-Control-Max-Age", maxAge)
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}
