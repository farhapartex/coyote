package auth

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"
)

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func newUserID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
