package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/session"
)

var (
	ErrInvalidCredentials = errors.New("coyote/auth: invalid username or password")
	ErrInactiveAccount    = errors.New("coyote/auth: account is disabled")
)

type contextKey struct{}

var userContextKey contextKey

type Options struct {
	Hasher            Hasher
	MinPasswordLength int
}

type Service struct {
	users             Store
	sessions          *session.Manager
	hasher            Hasher
	minPasswordLength int
}

func NewService(users Store, sessions *session.Manager, opts Options) *Service {
	if users == nil {
		users = NewMemoryStore()
	}
	if opts.MinPasswordLength < 1 {
		opts.MinPasswordLength = DefaultMinPasswordLen
	}
	return &Service{
		users:             users,
		sessions:          sessions,
		hasher:            opts.Hasher,
		minPasswordLength: opts.MinPasswordLength,
	}
}

func (s *Service) Users() Store { return s.users }

func (s *Service) MinPasswordLength() int { return s.minPasswordLength }

func (s *Service) ValidatePassword(password string) error {
	return ValidatePasswordLength(password, s.minPasswordLength)
}

func (s *Service) HashPassword(password string) (string, error) {
	return s.hasher.Hash(password)
}

type NewUser struct {
	Username     string
	Email        string
	FirstName    string
	LastName     string
	Password     string
	IsSuperadmin bool
}

func (s *Service) CreateUser(in NewUser) (*User, error) {
	if err := s.ValidatePassword(in.Password); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	u := &User{
		FirstName:    strings.TrimSpace(in.FirstName),
		LastName:     strings.TrimSpace(in.LastName),
		Email:        strings.TrimSpace(in.Email),
		Username:     strings.TrimSpace(in.Username),
		Password:     hash,
		IsActive:     true,
		IsSuperadmin: in.IsSuperadmin,
		CreatedAt:    time.Now(),
	}
	if err := s.users.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) CreateSuperadmin(username, email, password string) (*User, error) {
	return s.CreateUser(NewUser{
		Username:     username,
		Email:        email,
		Password:     password,
		IsSuperadmin: true,
	})
}

func (s *Service) SetPassword(id, password string) error {
	if err := s.ValidatePassword(password); err != nil {
		return err
	}
	u, err := s.users.ByID(id)
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	u.Password = hash
	return s.users.Update(u)
}

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

func (s *Service) RequireLogin(loginURL string) func(http.Handler) http.Handler {
	return s.guard(loginURL, func(u *User) bool { return u != nil })
}

func (s *Service) RequireSuperadmin(loginURL string) func(http.Handler) http.Handler {
	return s.guard(loginURL, func(u *User) bool { return u != nil && u.IsSuperadmin })
}

func (s *Service) guard(loginURL string, allow func(*User) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := s.CurrentUser(r)
			if allow(u) {
				next.ServeHTTP(w, r)
				return
			}
			if u != nil {
				http.Error(w, "403 forbidden", http.StatusForbidden)
				return
			}
			target := loginURL
			if target == "" {
				http.Error(w, "401 unauthorized", http.StatusUnauthorized)
				return
			}
			redirect := url.URL{Path: target}
			query := redirect.Query()
			query.Set("next", r.URL.RequestURI())
			redirect.RawQuery = query.Encode()
			http.Redirect(w, r, redirect.String(), http.StatusSeeOther)
		})
	}
}
