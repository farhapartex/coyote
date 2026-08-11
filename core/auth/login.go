package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/session"
)

func (s *Service) Authenticate(username, password string) (*User, error) {
	u, err := s.users.ByUsername(username)
	if err != nil {
		_, _ = s.hasher.Hash(password)
		return nil, ErrInvalidCredentials
	}
	if !VerifyPassword(password, u.Password) {
		return nil, ErrInvalidCredentials
	}
	if !u.IsActive {
		return nil, ErrInactiveAccount
	}
	return u, nil
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
	u.LastLoginAt = time.Now()
	if err := s.users.Update(u); err != nil {
		return err
	}
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
	u, err := s.users.ByID(id)
	if err != nil || !u.IsActive {
		sess.ClearUser()
		return nil
	}
	return u
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := s.loadUser(r); u != nil {
			r = r.WithContext(context.WithValue(r.Context(), userContextKey, u))
		}
		next.ServeHTTP(w, r)
	})
}

func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}
