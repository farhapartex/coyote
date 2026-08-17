package app

import (
	"log/slog"

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

func permissionStore(s Settings, a *App) auth.PermissionStore {
	if s.Auth.PermissionStore != nil {
		return s.Auth.PermissionStore
	}
	if !s.Auth.Permissions || s.Database().Engine == "" {
		return nil
	}
	return store.LazyPermissions(a.DB)
}

func tokenStore(s Settings, a *App) auth.TokenStore {
	if !s.Auth.ResetTokens || s.Database().Engine == "" {
		return nil
	}
	return store.LazyTokens(a.DB)
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

func sessionCarrier(s Settings, a *App, logger *slog.Logger) (session.Store, *session.Sealer) {
	if !s.Sessions.Backend.Stateless() || s.Sessions.Store != nil {
		return sessionStore(s, a), nil
	}
	sealer, err := session.NewSealer(s.SecretKey)
	if err != nil {
		logger.Error("falling back to in-memory sessions", "error", err)
		return session.NewMemoryStore(s.Sessions.CleanupInterval), nil
	}
	return nil, sealer
}

func (a *App) AuthService() *auth.Service { return a.Auth }
