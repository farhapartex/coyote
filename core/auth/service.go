package auth

import (
	"errors"

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
	PasswordRules     []PasswordRule
	Throttle          ThrottlePolicy
	Limiter           LoginLimiter
	TrustProxy        bool
}

type Service struct {
	users             Store
	sessions          *session.Manager
	hasher            Hasher
	minPasswordLength int
	passwordRules     []PasswordRule
	limiter           LoginLimiter
	trustProxy        bool
}

func NewService(users Store, sessions *session.Manager, opts Options) *Service {
	if users == nil {
		users = NewMemoryStore()
	}
	users = Guarded(users)
	if opts.MinPasswordLength < 1 {
		opts.MinPasswordLength = DefaultMinPasswordLen
	}
	if opts.PasswordRules == nil {
		opts.PasswordRules = DefaultPasswordRules(opts.MinPasswordLength)
	}
	if opts.Limiter == nil && opts.Throttle.Enabled {
		opts.Limiter = NewLoginLimiter(opts.Throttle)
	}
	return &Service{
		users:             users,
		sessions:          sessions,
		hasher:            opts.Hasher,
		minPasswordLength: opts.MinPasswordLength,
		passwordRules:     opts.PasswordRules,
		limiter:           opts.Limiter,
		trustProxy:        opts.TrustProxy,
	}
}

func (s *Service) Users() Store { return s.users }

func (s *Service) MinPasswordLength() int { return s.minPasswordLength }

func (s *Service) PasswordRules() []PasswordRule { return s.passwordRules }

func (s *Service) ValidatePassword(password string) error {
	return s.ValidatePasswordFor(password, nil)
}

func (s *Service) ValidatePasswordFor(password string, user *User) error {
	return ApplyPasswordRules(s.passwordRules, password, user)
}

func (s *Service) HashPassword(password string) (string, error) {
	return s.hasher.Hash(password)
}
