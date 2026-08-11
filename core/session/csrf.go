package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

func (m *Manager) CSRFToken(r *http.Request) string {
	sess := FromRequest(r)
	if sess == nil {
		return ""
	}
	if token := sess.GetString(csrfKey); token != "" {
		return token
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	sess.Set(csrfKey, token)
	return token
}

func (m *Manager) ValidCSRF(r *http.Request, candidate string) bool {
	sess := FromRequest(r)
	if sess == nil || candidate == "" {
		return false
	}
	token := sess.GetString(csrfKey)
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1
}
