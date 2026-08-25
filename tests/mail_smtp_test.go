package tests

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/lib/mail"
)

func smtpOptions(server *fakeSMTP, mode mail.TLSMode) mail.SMTPOptions {
	return mail.SMTPOptions{
		Host:      server.Host(),
		Port:      server.Port(),
		TLS:       mode,
		From:      "shop@example.test",
		Timeout:   5 * time.Second,
		LocalName: "test.example",
		TLSConfig: server.ClientTLS(),
	}
}

func commandsOf(record smtpConversation) string {
	return strings.Join(record.commands, " | ")
}

func TestSMTPFullConversation(t *testing.T) {
	server := newFakeSMTP(t, "SIZE 35882577", "8BITMIME")
	sender := mail.NewSMTP(smtpOptions(server, mail.TLSNone))

	if sender.Addr() != fmt.Sprintf("%s:%d", server.Host(), server.Port()) {
		t.Errorf("Addr = %q", sender.Addr())
	}

	message := mail.Message{
		From:    "Shop <shop@example.test>",
		To:      []string{"ada@example.test"},
		Subject: "Your order",
		Text:    "Thank you.",
	}

	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, ok := server.Last()
	if !ok {
		t.Fatal("the server recorded no conversation")
	}

	joined := commandsOf(record)
	for _, want := range []string{
		"EHLO test.example",
		"MAIL FROM:<shop@example.test>",
		"RCPT TO:<ada@example.test>",
		"DATA",
		"QUIT",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("%q is missing from the conversation: %s", want, joined)
		}
	}

	if !strings.Contains(record.data, "Subject: Your order") {
		t.Errorf("the body did not arrive:\n%s", record.data)
	}
	if !strings.HasSuffix(record.data, "\r\n") {
		t.Error("the message should end with a line break")
	}
}

func TestSMTPEnvelopeCarriesBccButTheBodyDoesNot(t *testing.T) {
	server := newFakeSMTP(t)
	sender := mail.NewSMTP(smtpOptions(server, mail.TLSNone))

	message := mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		CC:      []string{"carol@example.test"},
		BCC:     []string{"secret@example.test"},
		Subject: "Your order",
		Text:    "Thank you.",
	}

	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, _ := server.Last()
	joined := commandsOf(record)

	for _, want := range []string{
		"RCPT TO:<ada@example.test>",
		"RCPT TO:<carol@example.test>",
		"RCPT TO:<secret@example.test>",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the envelope is missing %q: %s", want, joined)
		}
	}

	if strings.Contains(record.data, "secret@example.test") {
		t.Errorf("the blind copy leaked into the body:\n%s", record.data)
	}
	if strings.Contains(strings.ToLower(record.data), "bcc:") {
		t.Error("no Bcc header should reach the wire")
	}
	if !strings.Contains(record.data, "carol@example.test") {
		t.Error("a visible copy should appear in the body")
	}
}

func TestSMTPStartTLSUpgradesAndCarriesTheMessage(t *testing.T) {
	server := newFakeSMTP(t, "STARTTLS", "AUTH PLAIN")
	sender := mail.NewSMTP(smtpOptions(server, mail.TLSStartTLS))

	if err := sender.Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, _ := server.Last()
	if !record.upgradedToTLS {
		t.Fatal("the connection was never upgraded")
	}
	if !strings.Contains(record.data, "Subject: Your order") {
		t.Error("the message did not travel over the upgraded connection")
	}

	joined := commandsOf(record)
	if strings.Count(joined, "EHLO") != 2 {
		t.Errorf("EHLO should be repeated after STARTTLS: %s", joined)
	}
}

func TestSMTPRefusesWhenStartTLSIsDemandedButNotOffered(t *testing.T) {
	server := newFakeSMTP(t, "8BITMIME")
	sender := mail.NewSMTP(smtpOptions(server, mail.TLSStartTLS))

	err := sender.Send(context.Background(), validMessage())
	if !errors.Is(err, mail.ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("the error should name STARTTLS: %v", err)
	}

	record, _ := server.Last()
	joined := commandsOf(record)
	if strings.Contains(joined, "MAIL FROM") || strings.Contains(joined, "DATA") {
		t.Errorf("nothing should be sent in clear: %s", joined)
	}
	if record.data != "" {
		t.Errorf("a body reached a cleartext server:\n%s", record.data)
	}
}

func TestSMTPPlainAuthOverTLS(t *testing.T) {
	server := newFakeSMTP(t, "STARTTLS", "AUTH PLAIN LOGIN")

	options := smtpOptions(server, mail.TLSStartTLS)
	options.Username = "shop"
	options.Password = "s3cret"

	if err := mail.NewSMTP(options).Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, _ := server.Last()
	if !record.authenticated {
		t.Fatal("the server never saw authentication")
	}
	if !record.upgradedToTLS {
		t.Fatal("credentials must only cross an encrypted link")
	}
	if len(record.credentials) != 1 {
		t.Fatalf("credentials = %v", record.credentials)
	}

	decoded, err := base64.StdEncoding.DecodeString(record.credentials[0])
	if err != nil {
		t.Fatalf("decoding the PLAIN token: %v", err)
	}
	if !strings.Contains(string(decoded), "shop") || !strings.Contains(string(decoded), "s3cret") {
		t.Errorf("the PLAIN token does not carry the credentials: %q", decoded)
	}
}

func TestSMTPLoginAuthOverTLS(t *testing.T) {
	server := newFakeSMTP(t, "STARTTLS", "AUTH LOGIN")

	options := smtpOptions(server, mail.TLSStartTLS)
	options.Username = "shop"
	options.Password = "s3cret"

	if err := mail.NewSMTP(options).Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, _ := server.Last()
	if !record.authenticated || !record.upgradedToTLS {
		t.Fatalf("LOGIN did not complete over TLS: %+v", record)
	}
	if len(record.credentials) != 2 {
		t.Fatalf("credentials = %v, want a username and a password", record.credentials)
	}

	for i, want := range []string{"shop", "s3cret"} {
		decoded, err := base64.StdEncoding.DecodeString(record.credentials[i])
		if err != nil {
			t.Fatalf("decoding %q: %v", record.credentials[i], err)
		}
		if string(decoded) != want {
			t.Errorf("credential %d = %q, want %q", i, decoded, want)
		}
	}

	if !strings.Contains(record.data, "Subject: Your order") {
		t.Error("the message did not arrive after LOGIN")
	}
}

func TestSMTPAuthenticatesOverImplicitTLS(t *testing.T) {
	for _, advertised := range []string{"AUTH PLAIN", "AUTH LOGIN"} {
		t.Run(advertised, func(t *testing.T) {
			server := newFakeSMTPWithImplicitTLS(t, advertised)

			options := smtpOptions(server, mail.TLSImplicit)
			options.Username = "shop"
			options.Password = "s3cret"

			if err := mail.NewSMTP(options).Send(context.Background(), validMessage()); err != nil {
				t.Fatalf("Send over implicit TLS: %v", err)
			}

			record, _ := server.Last()
			if !record.upgradedToTLS {
				t.Fatal("the fake server did not run TLS")
			}
			if !record.authenticated {
				t.Error("authentication should succeed: the connection is already encrypted")
			}
			if !strings.Contains(record.data, "Subject: Your order") {
				t.Error("the message did not arrive")
			}
			if strings.Contains(commandsOf(record), "STARTTLS") {
				t.Error("implicit TLS needs no STARTTLS")
			}
		})
	}
}

func TestSMTPRefusesCredentialsOverACleartextConnection(t *testing.T) {
	server := newFakeSMTP(t, "AUTH PLAIN LOGIN")

	for _, mechanism := range []string{"AUTH PLAIN", "AUTH LOGIN"} {
		t.Run(mechanism, func(t *testing.T) {
			options := smtpOptions(server, mail.TLSNone)
			options.Username = "shop"
			options.Password = "s3cret"

			err := mail.NewSMTP(options).Send(context.Background(), validMessage())
			if !errors.Is(err, mail.ErrNotConfigured) {
				t.Fatalf("error = %v, want ErrNotConfigured", err)
			}
			if !strings.Contains(err.Error(), "unencrypted") {
				t.Errorf("the error should say why: %v", err)
			}
		})
	}

	for _, record := range server.Conversations() {
		if record.authenticated {
			t.Error("credentials crossed a cleartext connection")
		}
		if strings.Contains(commandsOf(record), "AUTH") {
			t.Errorf("an AUTH command was sent in clear: %s", commandsOf(record))
		}
	}
}

func TestSMTPOptionsRefuseCredentialsWithoutEncryption(t *testing.T) {
	err := mail.SMTPOptions{Host: "smtp.example.test", TLS: mail.TLSNone, Username: "u", Password: "p"}.Validate()
	if !errors.Is(err, mail.ErrNotConfigured) {
		t.Errorf("error = %v, want ErrNotConfigured", err)
	}

	if err := (mail.SMTPOptions{Host: "smtp.example.test", TLS: mail.TLSNone}).Validate(); err != nil {
		t.Errorf("an anonymous cleartext relay is a legitimate configuration: %v", err)
	}
	if err := (mail.SMTPOptions{TLS: mail.TLSStartTLS}).Validate(); !errors.Is(err, mail.ErrNotConfigured) {
		t.Error("an empty host should be refused")
	}
	if err := (mail.SMTPOptions{Host: "h", TLS: "sometimes"}).Validate(); !errors.Is(err, mail.ErrNotConfigured) {
		t.Error("an unknown TLS mode should be refused")
	}
	if err := (mail.SMTPOptions{Host: "h", Port: 70000}).Validate(); !errors.Is(err, mail.ErrNotConfigured) {
		t.Error("an impossible port should be refused")
	}
	if err := (mail.SMTPOptions{Host: "h", From: "a@b.c\r\nBcc: x@y.z"}).Validate(); !errors.Is(err, mail.ErrHeaderInjection) {
		t.Error("an injected From should be refused")
	}
}

func TestSMTPDefaultPorts(t *testing.T) {
	for mode, want := range map[mail.TLSMode]int{
		mail.TLSStartTLS: 587,
		mail.TLSImplicit: 465,
		mail.TLSNone:     25,
		"":               587,
	} {
		sender := mail.NewSMTP(mail.SMTPOptions{Host: "smtp.example.test", TLS: mode})
		if got := sender.Addr(); got != fmt.Sprintf("smtp.example.test:%d", want) {
			t.Errorf("mode %q gave %q, want port %d", mode, got, want)
		}
	}

	sender := mail.NewSMTP(mail.SMTPOptions{Host: "smtp.example.test", Port: 2525})
	if got := sender.Addr(); got != "smtp.example.test:2525" {
		t.Errorf("an explicit port should win, got %q", got)
	}
}

func TestSMTPServerErrorsSurfaceAsSendFailed(t *testing.T) {
	for name, tc := range map[string]struct {
		verb  string
		reply string
	}{
		"rejected recipient":  {verb: "RCPT", reply: "550 5.1.1 no such mailbox"},
		"unavailable at data": {verb: "DATA", reply: "421 4.3.2 service shutting down"},
		"rejected sender":     {verb: "MAIL", reply: "553 5.7.1 sender denied"},
	} {
		t.Run(name, func(t *testing.T) {
			server := newFakeSMTP(t)
			server.Reply(tc.verb, tc.reply)

			err := mail.NewSMTP(smtpOptions(server, mail.TLSNone)).Send(context.Background(), validMessage())
			if !errors.Is(err, mail.ErrSendFailed) {
				t.Fatalf("error = %v, want ErrSendFailed", err)
			}

			code := strings.Fields(tc.reply)[0]
			if !strings.Contains(err.Error(), code) {
				t.Errorf("the error should carry the server code %s: %v", code, err)
			}
		})
	}
}

func TestSMTPRefusesAMessageOverTheAdvertisedSize(t *testing.T) {
	server := newFakeSMTP(t, "SIZE 100")

	message := validMessage()
	message.Text = strings.Repeat("padding ", 500)

	err := mail.NewSMTP(smtpOptions(server, mail.TLSNone)).Send(context.Background(), message)
	if !errors.Is(err, mail.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}

	record, _ := server.Last()
	if strings.Contains(commandsOf(record), "DATA") {
		t.Error("an oversize message should not reach DATA")
	}
}

func TestSMTPTimesOutAgainstASilentServer(t *testing.T) {
	server := newFakeSMTP(t)
	server.Silence()

	options := smtpOptions(server, mail.TLSNone)
	options.Timeout = 300 * time.Millisecond

	started := time.Now()
	err := mail.NewSMTP(options).Send(context.Background(), validMessage())
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("a silent server should not produce a successful send")
	}
	if elapsed > 3*time.Second {
		t.Errorf("the send took %v, the timeout should have fired", elapsed)
	}
}

func TestSMTPFailsOnACancelledContext(t *testing.T) {
	server := newFakeSMTP(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mail.NewSMTP(smtpOptions(server, mail.TLSNone)).Send(ctx, validMessage())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(server.Conversations()) != 0 {
		t.Error("a cancelled send should not talk to the server")
	}
}

func TestSMTPRejectsAnInvalidMessageBeforeDialling(t *testing.T) {
	server := newFakeSMTP(t)
	sender := mail.NewSMTP(smtpOptions(server, mail.TLSNone))

	injected := validMessage()
	injected.Subject = "Hi\r\nBcc: attacker@evil.test"

	if err := sender.Send(context.Background(), injected); !errors.Is(err, mail.ErrHeaderInjection) {
		t.Errorf("error = %v, want ErrHeaderInjection", err)
	}
	if err := sender.Send(context.Background(), mail.Message{}); !errors.Is(err, mail.ErrNoRecipient) {
		t.Errorf("error = %v, want ErrNoRecipient", err)
	}
	if len(server.Conversations()) != 0 {
		t.Error("an invalid message should never reach the network")
	}
}

func TestSMTPFallsBackToTheConfiguredFrom(t *testing.T) {
	server := newFakeSMTP(t)

	options := smtpOptions(server, mail.TLSNone)
	options.From = "noreply@example.test"

	message := mail.Message{
		To:      []string{"ada@example.test"},
		Subject: "Your order",
		Text:    "Thank you.",
	}

	if err := mail.NewSMTP(options).Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, _ := server.Last()
	if !strings.Contains(commandsOf(record), "MAIL FROM:<noreply@example.test>") {
		t.Errorf("the configured From was not used: %s", commandsOf(record))
	}
	if !strings.Contains(record.data, "From: noreply@example.test") {
		t.Errorf("the header should carry it too:\n%s", record.data)
	}
}

func TestSMTPConcurrentSends(t *testing.T) {
	const sends = 20

	server := newFakeSMTP(t)
	sender := mail.NewSMTP(smtpOptions(server, mail.TLSNone))

	var wait sync.WaitGroup
	errs := make(chan error, sends)

	for i := 0; i < sends; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()

			message := validMessage()
			message.Subject = "Order " + strconv.Itoa(i)
			if err := sender.Send(context.Background(), message); err != nil {
				errs <- err
			}
		}(i)
	}

	wait.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("a concurrent send failed: %v", err)
	}

	conversations := server.Conversations()
	if len(conversations) != sends {
		t.Fatalf("the server held %d conversations, want %d", len(conversations), sends)
	}

	subjects := map[string]bool{}
	for _, record := range conversations {
		for _, line := range strings.Split(record.data, "\r\n") {
			if subject, found := strings.CutPrefix(line, "Subject: "); found {
				subjects[subject] = true
			}
		}
	}
	if len(subjects) != sends {
		t.Errorf("%d distinct messages arrived, want %d", len(subjects), sends)
	}
}

func TestSMTPAgainstARealServer(t *testing.T) {
	addr := os.Getenv("COYOTE_TEST_SMTP")
	if addr == "" {
		t.Skip("set COYOTE_TEST_SMTP=host:port to run against a real SMTP server")
	}

	host, port, found := strings.Cut(addr, ":")
	if !found {
		t.Fatalf("COYOTE_TEST_SMTP should be host:port, got %q", addr)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port %q: %v", port, err)
	}

	options := mail.SMTPOptions{
		Host:     host,
		Port:     number,
		TLS:      mail.TLSMode(os.Getenv("COYOTE_TEST_SMTP_TLS")),
		Username: os.Getenv("COYOTE_TEST_SMTP_USER"),
		Password: os.Getenv("COYOTE_TEST_SMTP_PASSWORD"),
		From:     "coyote@example.test",
		Timeout:  10 * time.Second,
	}
	if options.TLS == "" {
		options.TLS = mail.TLSNone
	}

	message := mail.Message{
		To:      []string{"ada@example.test"},
		Subject: "coyote SMTP backend check",
		Text:    "Sent by the coyote test suite.",
		HTML:    "<p>Sent by the <b>coyote</b> test suite.</p>",
	}

	if err := mail.NewSMTP(options).Send(context.Background(), message); err != nil {
		t.Fatalf("sending to %s: %v", addr, err)
	}
}
