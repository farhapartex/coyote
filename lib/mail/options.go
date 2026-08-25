package mail

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"time"
)

type TLSMode string

const (
	TLSNone     TLSMode = "none"
	TLSStartTLS TLSMode = "starttls"
	TLSImplicit TLSMode = "tls"
)

const (
	defaultSMTPPort     = 587
	defaultImplicitPort = 465
	defaultPlainPort    = 25
	defaultTimeout      = 30 * time.Second
	defaultLocalName    = "localhost"
)

type SMTPOptions struct {
	Host      string
	Port      int
	Username  string
	Password  string
	TLS       TLSMode
	From      string
	Timeout   time.Duration
	LocalName string
	TLSConfig *tls.Config
}

func (o SMTPOptions) withDefaults() SMTPOptions {
	if o.TLS == "" {
		o.TLS = TLSStartTLS
	}
	if o.Port == 0 {
		o.Port = defaultPortFor(o.TLS)
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.LocalName == "" {
		o.LocalName = defaultLocalName
	}
	return o
}

func defaultPortFor(mode TLSMode) int {
	switch mode {
	case TLSImplicit:
		return defaultImplicitPort
	case TLSNone:
		return defaultPlainPort
	default:
		return defaultSMTPPort
	}
}

func (o SMTPOptions) addr() string {
	return o.Host + ":" + strconv.Itoa(o.Port)
}

func (o SMTPOptions) tlsConfig() *tls.Config {
	if o.TLSConfig != nil {
		return o.TLSConfig.Clone()
	}
	return &tls.Config{ServerName: o.Host, MinVersion: tls.VersionTLS12}
}

func (o SMTPOptions) hasCredentials() bool {
	return o.Username != "" || o.Password != ""
}

func (o SMTPOptions) encrypted() bool {
	return o.TLS == TLSStartTLS || o.TLS == TLSImplicit
}

func (o SMTPOptions) Validate() error {
	if o.Host == "" {
		return fmt.Errorf("%w: the SMTP host is empty", ErrNotConfigured)
	}
	switch o.TLS {
	case "", TLSNone, TLSStartTLS, TLSImplicit:
	default:
		return fmt.Errorf("%w: %q is not a TLS mode, use none, starttls or tls",
			ErrNotConfigured, o.TLS)
	}
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("%w: %d is not a port", ErrNotConfigured, o.Port)
	}
	if o.hasCredentials() && o.TLS == TLSNone {
		return fmt.Errorf("%w: refusing to send SMTP credentials over an unencrypted connection",
			ErrNotConfigured)
	}
	if containsBreak(o.From) || containsBreak(o.Username) {
		return fmt.Errorf("%w: SMTP options carry a line break", ErrHeaderInjection)
	}
	return nil
}
