package session

import (
	"context"
	"net/http"
)

type contextKey struct{}

var sessionContextKey contextKey

func From(ctx context.Context) *Session {
	sess, _ := ctx.Value(sessionContextKey).(*Session)
	return sess
}

func FromRequest(r *http.Request) *Session {
	return From(r.Context())
}
