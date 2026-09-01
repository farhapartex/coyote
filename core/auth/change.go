package auth

import (
	"errors"
	"net/http"

	"github.com/farhapartex/coyote/core/session"
)

var (
	ErrPasswordMismatch = errors.New("coyote/auth: the new passwords do not match")
	ErrWrongPassword    = errors.New("coyote/auth: the current password is not right")
	ErrChangeDisabled   = errors.New("coyote/auth: password changes are turned off")
)

type PasswordChange struct {
	Current string
	New     string
	Confirm string
}

func (s *Service) AllowsPasswordChange() bool { return s.allowChange }

func (s *Service) ChangePassword(r *http.Request, in PasswordChange) error {
	if !s.allowChange {
		return ErrChangeDisabled
	}
	user := s.CurrentUser(r)
	if user == nil {
		return ErrInvalidCredentials
	}
	if in.New != in.Confirm {
		return ErrPasswordMismatch
	}

	key := loginKey(user.Username, s.clientBucket(r))
	if s.limiter != nil && !s.limiter.Allow(key) {
		return ErrTooManyAttempts
	}
	if !VerifyPassword(in.Current, user.Password) {
		s.recordFailure(key)
		return ErrWrongPassword
	}
	if s.limiter != nil {
		s.limiter.Reset(key)
	}

	if err := s.SetPassword(user.ID, in.New); err != nil {
		return err
	}
	return s.Login(r, user)
}

func (s *Service) ResetPassword(userID string, in PasswordChange) error {
	if in.New != in.Confirm {
		return ErrPasswordMismatch
	}
	return s.SetPassword(userID, in.New)
}

func (s *Service) RevokeOtherSessions(r *http.Request) int {
	current := session.FromRequest(r)
	if current == nil || s.sessions == nil {
		return 0
	}
	store, ok := s.sessions.Store().(session.ManageableStore)
	if !ok {
		return 0
	}

	userID := current.UserID()
	if userID == "" {
		return 0
	}
	removed := 0
	for _, other := range store.All() {
		if other.UserID() != userID || other.ID() == current.ID() {
			continue
		}
		if err := store.Delete(other.ID()); err == nil {
			removed++
		}
	}
	return removed
}
