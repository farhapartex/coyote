package settings

import (
	"fmt"
	"os"
	"time"

	"github.com/farhapartex/coyote/lib/mail"
)

type EmailBackend string

const (
	EmailToSMTP    EmailBackend = "smtp"
	EmailToConsole EmailBackend = "console"
	EmailToFile    EmailBackend = "file"
	EmailToMemory  EmailBackend = "memory"
)

const (
	DefaultEmailDir     = "mail"
	DefaultEmailTimeout = 10 * time.Second
)

type Email struct {
	Backend   EmailBackend
	Host      string
	Port      int
	Username  string
	Password  string
	TLS       mail.TLSMode
	From      string
	Dir       string
	Timeout   time.Duration
	LocalName string

	Sender mail.Sender
}

func (e Email) Enabled() bool {
	return e.Backend != "" || e.Sender != nil
}

func (e Email) Redacted() Email {
	copied := e
	if copied.Password != "" {
		copied.Password = "••••••"
	}
	return copied
}

func (e Email) Location() string {
	switch e.Backend {
	case EmailToSMTP:
		return mail.NewSMTP(e.smtpOptions()).Addr()
	case EmailToFile:
		return e.Dir
	case EmailToConsole:
		return "standard output"
	case EmailToMemory:
		return "in process"
	default:
		if e.Sender != nil {
			return fmt.Sprintf("%T", e.Sender)
		}
		return "not configured"
	}
}

func (e Email) Open() (mail.Sender, error) {
	if e.Sender != nil {
		return e.Sender, nil
	}

	switch e.Backend {
	case EmailToSMTP:
		return mail.NewSMTP(e.smtpOptions()), nil
	case EmailToConsole:
		return mail.NewConsole(os.Stdout), nil
	case EmailToFile:
		return mail.NewFile(e.Dir), nil
	case EmailToMemory:
		return mail.NewMemory(), nil
	default:
		return nil, mail.ErrNotConfigured
	}
}

func (e Email) smtpOptions() mail.SMTPOptions {
	return mail.SMTPOptions{
		Host:      e.Host,
		Port:      e.Port,
		Username:  e.Username,
		Password:  e.Password,
		TLS:       e.TLS,
		From:      e.From,
		Timeout:   e.Timeout,
		LocalName: e.LocalName,
	}
}
