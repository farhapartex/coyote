package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

const (
	MaxCookieSize = 4000
	keyLength     = 32
	derivationTag = "coyote/session/cookie/v1"
)

var (
	ErrSealedTooLarge = errors.New("coyote/session: sealed session exceeds the cookie size limit")
	ErrSealedInvalid  = errors.New("coyote/session: sealed session is missing, forged or expired")
)

type Sealer struct {
	aead cipher.AEAD
}

func NewSealer(secret string) (*Sealer, error) {
	if secret == "" {
		return nil, errors.New("coyote/session: cookie sessions need a SecretKey")
	}
	key, err := hkdf.Key(sha256.New, []byte(secret), nil, derivationTag, keyLength)
	if err != nil {
		return nil, fmt.Errorf("coyote/session: deriving key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("coyote/session: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("coyote/session: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

func (s *Sealer) Seal(payload []byte) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("coyote/session: %w", err)
	}
	sealed := s.aead.Seal(nonce, nonce, payload, nil)
	encoded := base64.RawURLEncoding.EncodeToString(sealed)
	if len(encoded) > MaxCookieSize {
		return "", fmt.Errorf("%w: %d bytes", ErrSealedTooLarge, len(encoded))
	}
	return encoded, nil
}

func (s *Sealer) Open(value string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrSealedInvalid
	}
	size := s.aead.NonceSize()
	if len(raw) < size {
		return nil, ErrSealedInvalid
	}
	payload, err := s.aead.Open(nil, raw[:size], raw[size:], nil)
	if err != nil {
		return nil, ErrSealedInvalid
	}
	return payload, nil
}
