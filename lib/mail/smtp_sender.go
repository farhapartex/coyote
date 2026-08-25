package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type SMTPSender struct {
	options SMTPOptions
	encoder Encoder
}

func NewSMTP(options SMTPOptions) *SMTPSender {
	return &SMTPSender{options: options.withDefaults(), encoder: NewEncoder()}
}

func (s *SMTPSender) Addr() string {
	return s.options.addr()
}

func (s *SMTPSender) Send(ctx context.Context, message Message) error {
	if err := s.options.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(message.From) == "" {
		message.From = s.options.From
	}

	envelope, err := s.envelope(message)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	conn, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	release := watchContext(ctx, conn)
	defer release()

	if err := s.deadline(ctx, conn); err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, s.options.Host)
	if err != nil {
		return s.fail(ctx, "greeting", err)
	}
	defer client.Close()

	if err := s.converse(ctx, client, envelope); err != nil {
		return err
	}
	if err := client.Quit(); err != nil {
		return s.fail(ctx, "QUIT", err)
	}
	return nil
}

type envelope struct {
	from       string
	recipients []string
	raw        []byte
}

func (s *SMTPSender) envelope(message Message) (envelope, error) {
	raw, err := s.encoder.Encode(message)
	if err != nil {
		return envelope{}, err
	}

	from, err := ParseAddress(message.From)
	if err != nil {
		return envelope{}, err
	}

	recipients := message.Recipients()
	if len(recipients) == 0 {
		return envelope{}, ErrNoRecipient
	}
	return envelope{from: from.Address, recipients: recipients, raw: raw}, nil
}

func (s *SMTPSender) converse(ctx context.Context, client *smtp.Client, e envelope) error {
	if err := client.Hello(s.options.LocalName); err != nil {
		return s.fail(ctx, "EHLO", err)
	}
	if err := s.upgrade(ctx, client); err != nil {
		return err
	}
	if err := s.authenticate(ctx, client); err != nil {
		return err
	}
	if err := s.checkSize(client, len(e.raw)); err != nil {
		return err
	}

	if err := client.Mail(e.from); err != nil {
		return s.fail(ctx, "MAIL FROM", err)
	}
	for _, recipient := range e.recipients {
		if err := client.Rcpt(recipient); err != nil {
			return s.fail(ctx, "RCPT TO "+recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return s.fail(ctx, "DATA", err)
	}
	if _, err := writer.Write(e.raw); err != nil {
		return s.fail(ctx, "writing the message", err)
	}
	if err := writer.Close(); err != nil {
		return s.fail(ctx, "ending the message", err)
	}
	return nil
}

func (s *SMTPSender) upgrade(ctx context.Context, client *smtp.Client) error {
	if s.options.TLS != TLSStartTLS {
		return nil
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("%w: %s does not offer STARTTLS and the configuration demands it",
			ErrNotConfigured, s.options.addr())
	}
	if err := client.StartTLS(s.options.tlsConfig()); err != nil {
		return s.fail(ctx, "STARTTLS", err)
	}
	return nil
}

func (s *SMTPSender) authenticate(ctx context.Context, client *smtp.Client) error {
	if !s.options.hasCredentials() {
		return nil
	}
	if !s.options.encrypted() {
		return fmt.Errorf("%w: refusing to authenticate over an unencrypted connection",
			ErrNotConfigured)
	}

	ok, mechanisms := client.Extension("AUTH")
	if !ok {
		return fmt.Errorf("%w: %s does not offer authentication", ErrSendFailed, s.options.addr())
	}

	auth, err := chooseAuth(s.options, mechanisms)
	if err != nil {
		return err
	}
	if err := client.Auth(auth); err != nil {
		return s.fail(ctx, "AUTH", err)
	}
	return nil
}

func (s *SMTPSender) checkSize(client *smtp.Client, size int) error {
	ok, declared := client.Extension("SIZE")
	if !ok {
		return nil
	}

	limit, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil || limit <= 0 {
		return nil
	}
	if size > limit {
		return fmt.Errorf("%w: the message is %d bytes and %s accepts at most %d",
			ErrTooLarge, size, s.options.addr(), limit)
	}
	return nil
}

func (s *SMTPSender) fail(ctx context.Context, stage string, err error) error {
	if cause := ctx.Err(); cause != nil {
		return cause
	}
	return fmt.Errorf("%w: %s at %s: %w", ErrSendFailed, stage, s.options.addr(), err)
}
