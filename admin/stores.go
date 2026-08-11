package admin

import (
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

func (a *Admin) sessionCount() int {
	if store, ok := a.sessionStore(); ok {
		return store.Count()
	}
	return -1
}

func (a *Admin) revokeUserSessions(userID string) {
	if store, ok := a.sessionStore(); ok {
		store.DeleteByUserID(userID)
	}
}
