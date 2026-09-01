package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/lib/clientip"
)

func (s *Service) Authenticate(ctx context.Context, username, password string) (*User, error) {
	return s.authenticate(ctx, loginKey(username, ""), username, password)
}

func (s *Service) AuthenticateRequest(r *http.Request, username, password string) (*User, error) {
	return s.authenticate(r.Context(), loginKey(username, s.clientBucket(r)), username, password)
}

func (s *Service) authenticate(ctx context.Context, key, username, password string) (*User, error) {
	if s.limiter != nil && !s.limiter.Allow(key) {
		return nil, ErrTooManyAttempts
	}

	u, err := s.users.ByUsername(ctx, username)
	if err != nil {
		_, _ = s.hasher.Hash(password)
		s.recordFailure(key)
		return nil, ErrInvalidCredentials
	}
	if !VerifyPassword(password, u.Password) {
		s.recordFailure(key)
		return nil, ErrInvalidCredentials
	}
	if !u.IsActive {
		s.recordFailure(key)
		return nil, ErrInactiveAccount
	}
	if s.limiter != nil {
		s.limiter.Reset(key)
	}
	return u, nil
}

func (s *Service) recordFailure(key string) {
	if s.limiter != nil {
		s.limiter.Fail(key)
	}
}

func (s *Service) clientBucket(r *http.Request) string {
	if r == nil {
		return ""
	}
	return clientip.Key(r, s.trustedProxies)
}

func loginKey(username, ip string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "|" + ip
}

func (s *Service) Login(r *http.Request, u *User) error {
	sess := session.FromRequest(r)
	if sess == nil {
		return session.ErrNoSession
	}
	if err := s.sessions.Renew(r); err != nil {
		return err
	}
	sess.SetUserID(u.ID)

	ctx := r.Context()
	stored, err := s.users.ByID(ctx, u.ID)
	if err != nil {
		return err
	}
	now := time.Now()
	stored.LastLoginAt = now
	if err := s.users.Update(ctx, stored); err != nil {
		return err
	}
	u.LastLoginAt = now
	return nil
}

func (s *Service) Logout(r *http.Request) error {
	return s.sessions.Destroy(r)
}

func (s *Service) CurrentUser(r *http.Request) *User {
	if u, ok := r.Context().Value(userContextKey).(*User); ok {
		return u
	}
	return s.loadUser(r)
}

func (s *Service) loadUser(r *http.Request) *User {
	sess := session.FromRequest(r)
	if sess == nil {
		return nil
	}
	id := sess.UserID()
	if id == "" {
		return nil
	}
	u, err := s.users.ByID(r.Context(), id)
	if err != nil || !u.IsActive {
		sess.ClearUser()
		return nil
	}
	return u
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := s.loadUser(r); u != nil {
			ctx := context.WithValue(r.Context(), userContextKey, u)
			r = r.WithContext(withPermissionCache(ctx, u.ID))
		}
		next.ServeHTTP(w, r)
	})
}

func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}
