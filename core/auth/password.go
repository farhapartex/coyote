package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	pbkdf2Algorithm        = "pbkdf2_sha256"
	DefaultIterations      = 600000
	DefaultMinPasswordLen  = 8
	MaxPasswordLength      = 1024
	pbkdf2KeyLength        = 32
	saltLength             = 16
	minSupportedIterations = 1000
)

var (
	ErrInvalidHash      = errors.New("coyote/auth: malformed password hash")
	ErrPasswordTooShort = errors.New("coyote/auth: password is too short")
	ErrPasswordTooLong  = errors.New("coyote/auth: password is too long")
	ErrPasswordRejected = errors.New("coyote/auth: password rejected")
)

type Hasher struct {
	Iterations int
}

func DefaultHasher() Hasher {
	return Hasher{Iterations: DefaultIterations}
}

func (h Hasher) iterations() int {
	if h.Iterations < minSupportedIterations {
		return DefaultIterations
	}
	return h.Iterations
}

func (h Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	iterations := h.iterations()
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, pbkdf2KeyLength)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s",
		pbkdf2Algorithm,
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (h Hasher) NeedsRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Algorithm {
		return true
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return true
	}
	return iterations < h.iterations()
}

func HashPassword(password string) (string, error) {
	return DefaultHasher().Hash(password)
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Algorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func LooksHashed(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Algorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	return err == nil && iterations > 0
}

func ValidatePassword(password string) error {
	return ValidatePasswordLength(password, DefaultMinPasswordLen)
}

func ValidatePasswordLength(password string, minLength int) error {
	if minLength < 1 {
		minLength = DefaultMinPasswordLen
	}
	if utf8.RuneCountInString(password) < minLength {
		return fmt.Errorf("%w: minimum is %d characters", ErrPasswordTooShort, minLength)
	}
	if len(password) > MaxPasswordLength {
		return fmt.Errorf("%w: maximum is %d bytes", ErrPasswordTooLong, MaxPasswordLength)
	}
	return nil
}
