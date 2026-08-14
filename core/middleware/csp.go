package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

const (
	CSPHeader           = "Content-Security-Policy"
	CSPReportOnlyHeader = "Content-Security-Policy-Report-Only"
	NoncePlaceholder    = "{nonce}"
)

type nonceKey struct{}

func CSP(policy string, reportOnly bool) Middleware {
	header := CSPHeader
	if reportOnly {
		header = CSPReportOnlyHeader
	}
	needsNonce := strings.Contains(policy, NoncePlaceholder)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !needsNonce {
				w.Header().Set(header, policy)
				next.ServeHTTP(w, r)
				return
			}
			nonce := newNonce()
			w.Header().Set(header, strings.ReplaceAll(policy, NoncePlaceholder, "'nonce-"+nonce+"'"))
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), nonceKey{}, nonce)))
		})
	}
}

func NonceFrom(ctx context.Context) string {
	value, _ := ctx.Value(nonceKey{}).(string)
	return value
}

func newNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
