package admin

import (
	"context"
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
)

func (a *Admin) currentUser(r *http.Request) *auth.User {
	return a.app.Auth.CurrentUser(r)
}

func (a *Admin) sessionStore() (session.ManageableStore, bool) {
	return a.app.ManageableSessions()
}

func (a *Admin) sessionCount(ctx context.Context) int {
	store, ok := a.sessionStore()
	if !ok {
		return -1
	}
	total, err := store.Count(ctx)
	if err != nil {
		return -1
	}
	return total
}

func (a *Admin) revokeUserSessions(ctx context.Context, userID string) {
	if store, ok := a.sessionStore(); ok {
		_, _ = store.DeleteByUserID(ctx, userID)
	}
}
