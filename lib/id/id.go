package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func New() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("coyote/id: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}

func MustNew() string {
	value, err := New()
	if err != nil {
		panic(err)
	}
	return value
}

func Short() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("coyote/id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func MustShort() string {
	value, err := Short()
	if err != nil {
		panic(err)
	}
	return value
}
