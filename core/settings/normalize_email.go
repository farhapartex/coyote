package settings

import (
	"path/filepath"

	"github.com/farhapartex/coyote/lib/mail"
)

func (s *Settings) normalizeEmail() {
	email := &s.Email
	if !email.Enabled() {
		return
	}

	if email.Timeout == 0 {
		email.Timeout = DefaultEmailTimeout
	}

	switch email.Backend {
	case EmailToFile:
		if email.Dir == "" {
			email.Dir = DefaultEmailDir
		}
		if !filepath.IsAbs(email.Dir) {
			email.Dir = filepath.Join(s.BaseDir, email.Dir)
		}
	case EmailToSMTP:
		if email.TLS == "" {
			email.TLS = mail.TLSStartTLS
		}
		if email.Port == 0 {
			email.Port = defaultEmailPort(email.TLS)
		}
	}
}

func defaultEmailPort(mode mail.TLSMode) int {
	switch mode {
	case mail.TLSImplicit:
		return 465
	case mail.TLSNone:
		return 25
	default:
		return 587
	}
}
