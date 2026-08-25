package settings

import (
	"strconv"

	"github.com/farhapartex/coyote/lib/mail"
)

func (s Settings) validateEmail(add func(string)) {
	email := s.Email

	if email.Sender != nil && email.Backend != "" {
		add("Email.Sender is set alongside the " + strconv.Quote(string(email.Backend)) +
			" backend; choose one")
	}
	if !email.Enabled() {
		s.validateUnusedEmailFields(add)
		return
	}

	if email.From != "" {
		if _, err := mail.ParseAddress(email.From); err != nil {
			add("Email.From " + strconv.Quote(email.From) + " is not a usable address")
		}
	}
	if email.Timeout < 0 {
		add("Email.Timeout cannot be negative")
	}

	switch email.Backend {
	case EmailToSMTP:
		s.validateSMTPEmail(add)
	case EmailToFile:
		if email.Dir == "" {
			add("Email.Dir is empty; the file backend needs a directory to write .eml files into")
		}
		s.validateEmailHasNoServer(add)
	case EmailToConsole, EmailToMemory:
		s.validateEmailHasNoServer(add)
	case "":
	default:
		add("Email.Backend " + strconv.Quote(string(email.Backend)) +
			" is not supported; use \"smtp\", \"console\", \"file\" or \"memory\"")
	}
}

func (s Settings) validateSMTPEmail(add func(string)) {
	email := s.Email

	if email.Host == "" {
		add("Email.Host is empty; the smtp backend needs a host")
	}
	if email.Port < 0 || email.Port > 65535 {
		add("Email.Port " + strconv.Itoa(email.Port) + " is not a port; use 1 to 65535")
	}

	switch email.TLS {
	case "", mail.TLSNone, mail.TLSStartTLS, mail.TLSImplicit:
	default:
		add("Email.TLS " + strconv.Quote(string(email.TLS)) +
			" is not supported; use \"none\", \"starttls\" or \"tls\"")
	}

	if (email.Username != "" || email.Password != "") && email.TLS == mail.TLSNone {
		add("Email.Username and Email.Password are set with Email.TLS \"none\"; " +
			"credentials would cross the network in clear, so use \"starttls\" or \"tls\"")
	}
}

func (s Settings) validateEmailHasNoServer(add func(string)) {
	email := s.Email
	backend := strconv.Quote(string(email.Backend))

	if email.Host != "" {
		add("Email.Host is set but the " + backend + " backend does not connect to a server")
	}
	if email.Username != "" || email.Password != "" {
		add("Email.Username and Email.Password are set but the " + backend +
			" backend does not authenticate")
	}
	if email.TLS != "" {
		add("Email.TLS is set but the " + backend + " backend does not use TLS")
	}
}

func (s Settings) validateUnusedEmailFields(add func(string)) {
	email := s.Email

	if email.Host != "" || email.Username != "" || email.Password != "" || email.Dir != "" {
		add("Email.Backend is empty but other Email fields are set; " +
			"set Email.Backend to \"smtp\", \"console\", \"file\" or \"memory\" to send email")
	}
}
