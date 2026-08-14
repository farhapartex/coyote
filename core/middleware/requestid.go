package middleware

import (
	"context"
	"net/http"
	"regexp"

	"github.com/farhapartex/coyote/lib/id"
)

const RequestIDHeader = "X-Request-Id"

type requestIDKey struct{}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestID(next http.Handler) http.Handler {
	return requestID(false)(next)
}

func TrustedRequestID(next http.Handler) http.Handler {
	return requestID(true)(next)
}

func requestID(trustInbound bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value := ""
			if trustInbound {
				if inbound := r.Header.Get(RequestIDHeader); safeRequestID.MatchString(inbound) {
					value = inbound
				}
			}
			if value == "" {
				value = id.MustShort()
			}
			w.Header().Set(RequestIDHeader, value)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, value)))
		})
	}
}

func RequestIDFrom(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}
