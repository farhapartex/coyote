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
