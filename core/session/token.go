package session

import (
	"encoding/json"
	"net/http"
)

type Token struct {
	manager *Manager
	request *http.Request
}

func (m *Manager) Token(r *http.Request) Token {
	return Token{manager: m, request: r}
}

func (t Token) String() string {
	if t.manager == nil || t.request == nil {
		return ""
	}
	return t.manager.CSRFToken(t.request)
}

func (t Token) Minted() string {
	if t.request == nil {
		return ""
	}
	sess := FromRequest(t.request)
	if sess == nil {
		return ""
	}
	return sess.GetString(csrfKey)
}

func (t Token) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}
