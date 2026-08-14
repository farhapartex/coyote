package app

import (
	"github.com/farhapartex/coyote/core/auth"
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

func (a *App) AuthService() *auth.Service { return a.Auth }
