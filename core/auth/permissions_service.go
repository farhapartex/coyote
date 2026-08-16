package auth

import (
	"context"
	"net/http"
	"sync"
)

type permissionCache struct {
	once  sync.Once
	set   map[string]struct{}
	err   error
	owner string
}

type permissionKey struct{}

var permissionContextKey permissionKey

func (s *Service) Permissions() PermissionStore { return s.permissions }

func (s *Service) Can(r *http.Request, codename string) bool {
	user := s.CurrentUser(r)
	if user == nil || !user.IsActive {
		return false
	}
	if user.IsSuperadmin {
		return true
	}
	granted, err := s.codenames(r, user)
	if err != nil {
		return false
	}
	_, found := granted[codename]
	return found
}

func (s *Service) CanAny(r *http.Request, codenames ...string) bool {
	for _, codename := range codenames {
		if s.Can(r, codename) {
			return true
		}
	}
	return false
}

func (s *Service) GrantedTo(userID string) ([]string, error) {
	if s.permissions == nil {
		return nil, nil
	}
	return s.permissions.CodenamesForUser(userID)
}

func (s *Service) codenames(r *http.Request, user *User) (map[string]struct{}, error) {
	if s.permissions == nil {
		return nil, nil
	}
	cache, _ := r.Context().Value(permissionContextKey).(*permissionCache)
	if cache == nil || cache.owner != user.ID {
		cache = &permissionCache{owner: user.ID}
	}
	cache.once.Do(func() {
		granted, err := s.permissions.CodenamesForUser(user.ID)
		if err != nil {
			cache.err = err
			return
		}
		cache.set = make(map[string]struct{}, len(granted))
		for _, codename := range granted {
			cache.set[codename] = struct{}{}
		}
	})
	return cache.set, cache.err
}

func (s *Service) RequirePermission(codename string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.Can(r, codename) {
				next.ServeHTTP(w, r)
				return
			}
			if s.CurrentUser(r) == nil {
				http.Error(w, "401 unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "403 forbidden", http.StatusForbidden)
		})
	}
}

func withPermissionCache(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, permissionContextKey, &permissionCache{owner: userID})
}
