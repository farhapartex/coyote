package mail

import (
	"fmt"
	"net/smtp"
	"strings"
)

type plainAuthenticator struct {
	username string
	password string
	host     string
}

func (a *plainAuthenticator) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if err := guardCredentials("PLAIN", a.host, server); err != nil {
		return "", nil, err
	}
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a *plainAuthenticator) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("%w: unexpected PLAIN challenge %q", ErrSendFailed, fromServer)
	}
	return nil, nil
}

type loginAuthenticator struct {
	username string
	password string
	host     string
}

func (a *loginAuthenticator) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if err := guardCredentials("LOGIN", a.host, server); err != nil {
		return "", nil, err
	}
	return "LOGIN", nil, nil
}

func (a *loginAuthenticator) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	switch challenge := strings.ToLower(strings.TrimSpace(string(fromServer))); {
	case strings.HasPrefix(challenge, "user"):
		return []byte(a.username), nil
	case strings.HasPrefix(challenge, "pass"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("%w: unexpected LOGIN challenge %q", ErrSendFailed, fromServer)
	}
}

func guardCredentials(mechanism, host string, server *smtp.ServerInfo) error {
	if !server.TLS {
		return fmt.Errorf("%w: refusing to send %s credentials over an unencrypted connection",
			ErrNotConfigured, mechanism)
	}
	if server.Name != host {
		return fmt.Errorf("%w: the server named itself %q, not %q",
			ErrNotConfigured, server.Name, host)
	}
	return nil
}

func chooseAuth(options SMTPOptions, advertised string) (smtp.Auth, error) {
	mechanisms := strings.ToUpper(advertised)

	switch {
	case strings.Contains(mechanisms, "PLAIN"):
		return &plainAuthenticator{
			username: options.Username,
			password: options.Password,
			host:     options.Host,
		}, nil
	case strings.Contains(mechanisms, "LOGIN"):
		return &loginAuthenticator{
			username: options.Username,
			password: options.Password,
			host:     options.Host,
		}, nil
	default:
		return nil, fmt.Errorf("%w: the server offers no authentication this client supports (%q)",
			ErrSendFailed, advertised)
	}
}
