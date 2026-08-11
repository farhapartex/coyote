package settings

import (
	"crypto/rand"
	"encoding/base64"
)

func generateSecretKey() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func GenerateSecretKey() (string, error) { return generateSecretKey() }
