package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTokenInvalid = errors.New("coyote/auth: reset token is unknown, used or expired")
	ErrNoTokenStore = errors.New("coyote/auth: no reset token store configured")
)

type ResetToken struct {
	Digest    string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"index;size:64;not null"`
	CreatedAt time.Time
	ExpiresAt time.Time `gorm:"index"`
	UsedAt    *time.Time
}

func (ResetToken) TableName() string { return "password_reset_tokens" }

type TokenStore interface {
	Save(token ResetToken) error
	ByDigest(digest string) (ResetToken, error)
	MarkUsed(digest string, at time.Time) error
	DeleteForUser(userID string) error
	Sweep(before time.Time) (int, error)
}

func TokenDigest(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Tokens() TokenStore { return s.tokens }

func (s *Service) CreateResetToken(userID string) (string, error) {
	if s.tokens == nil {
		return "", ErrNoTokenStore
	}
	if _, err := s.users.ByID(userID); err != nil {
		return "", err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("coyote/auth: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)

	lifetime := s.tokenLifetime
	if lifetime <= 0 {
		lifetime = time.Hour
	}
	now := time.Now()
	token := ResetToken{
		Digest:    TokenDigest(plain),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(lifetime),
	}
	if err := s.tokens.Save(token); err != nil {
		return "", err
	}
	return plain, nil
}

func (s *Service) CheckResetToken(plain string) (*User, error) {
	if s.tokens == nil {
		return nil, ErrNoTokenStore
	}
	token, err := s.tokens.ByDigest(TokenDigest(plain))
	if err != nil {
		return nil, ErrTokenInvalid
	}
	if token.UsedAt != nil || time.Now().After(token.ExpiresAt) {
		return nil, ErrTokenInvalid
	}
	user, err := s.users.ByID(token.UserID)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	return user, nil
}

func (s *Service) UseResetToken(plain string, in PasswordChange) (*User, error) {
	user, err := s.CheckResetToken(plain)
	if err != nil {
		return nil, err
	}
	if in.New != in.Confirm {
		return nil, ErrPasswordMismatch
	}
	if err := s.SetPassword(user.ID, in.New); err != nil {
		return nil, err
	}
	if err := s.tokens.MarkUsed(TokenDigest(plain), time.Now()); err != nil {
		return nil, err
	}
	return user, nil
}
