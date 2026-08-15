package app

import (
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/store"
)

func userStore(s Settings, a *App) auth.Store {
	if s.Auth.UserStore != nil {
		return s.Auth.UserStore
	}
	if s.Database().Engine == "" {
		return auth.NewMemoryStore()
	}
	return store.LazyUsers(a.DB)
}

func sessionStore(s Settings, a *App) session.Store {
	if s.Sessions.Store != nil {
		return s.Sessions.Store
	}
	if s.Sessions.Backend.Persistent() {
		return store.LazySessions(a.DB, s.Sessions.CleanupInterval)
	}
	return session.NewMemoryStore(s.Sessions.CleanupInterval)
}

func (a *App) AuthService() *auth.Service { return a.Auth }
