package tests

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/view"
	"github.com/farhapartex/coyote/lib/mail"
)

type contactApp struct {
	app    *app.App
	client *client
	sent   *mail.MemorySender
	log    *strings.Builder
}

func newContactApp(t *testing.T, sender mail.Sender) *contactApp {
	t.Helper()

	memory, _ := sender.(*mail.MemorySender)
	logged := &strings.Builder{}

	a := newTestApp(t, func(s *settings.Settings) {
		s.Email = settings.Email{From: "shop@example.test", Sender: sender}
	})

	a.Get("/contact", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/form.html", view.Data{"Title": "Contact"})
	}, a.CSRF).Named("contact")

	a.Post("/contact", contactHandler(a, sender, logged), a.CSRF)

	return &contactApp{app: a, client: newClient(t, a.Handler()), sent: memory, log: logged}
}

func contactHandler(a *app.App, sender mail.Sender, logged *strings.Builder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		message, err := enquiryFrom(r, a.Settings.Email.From)
		if err != nil {
			view.Flash(r, "error", "We could not prepare your message.")
			view.Redirect(w, r, "/contact")
			return
		}

		if err := sender.Send(r.Context(), message); err != nil {
			logged.WriteString(err.Error())
			view.Flash(r, "error", "We could not send your message.")
			view.Redirect(w, r, "/contact")
			return
		}

		view.Flash(r, "success", "Thank you, your message is on its way.")
		view.Redirect(w, r, "/contact")
	}
}

func enquiryFrom(r *http.Request, from string) (mail.Message, error) {
	name := strings.TrimSpace(r.PostForm.Get("name"))
	address := strings.TrimSpace(r.PostForm.Get("email"))
	body := strings.TrimSpace(r.PostForm.Get("message"))

	if name == "" || address == "" || body == "" {
		return mail.Message{}, errors.New("the enquiry is incomplete")
	}

	message := mail.Message{
		From:    from,
		To:      []string{from},
		ReplyTo: address,
		Subject: "Enquiry from " + name,
		Text:    name + " <" + address + "> wrote:\n\n" + body + "\n",
	}
	if err := message.Validate(); err != nil {
		return mail.Message{}, err
	}
	return message, nil
}

func (c *contactApp) submit(form url.Values) *http.Response {
	c.client.t.Helper()

	form.Set("csrf_token", c.client.token("/contact"))
	return c.client.do(http.MethodPost, "/contact", form).Result()
}

func validEnquiry() url.Values {
	return url.Values{
		"name":    {"Ada"},
		"email":   {"ada@example.test"},
		"message": {"Does the seam work?"},
	}
}

func TestContactFormSendsOneMessage(t *testing.T) {
	contact := newContactApp(t, mail.NewMemory())

	response := contact.submit(validEnquiry())
	if response.StatusCode != http.StatusSeeOther && response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want a redirect", response.StatusCode)
	}

	if contact.sent.Count() != 1 {
		t.Fatalf("the backend holds %d messages, want 1", contact.sent.Count())
	}

	message, _ := contact.sent.Last()
	if message.From != "shop@example.test" {
		t.Errorf("From = %q", message.From)
	}
	if len(message.To) != 1 || message.To[0] != "shop@example.test" {
		t.Errorf("To = %v", message.To)
	}
	if message.ReplyTo != "ada@example.test" {
		t.Errorf("ReplyTo = %q, the visitor should be replyable", message.ReplyTo)
	}
	if message.Subject != "Enquiry from Ada" {
		t.Errorf("Subject = %q", message.Subject)
	}
	if !strings.Contains(message.Text, "Does the seam work?") {
		t.Errorf("Text = %q", message.Text)
	}

	raw := string(contact.sent.Raw()[0])
	if !strings.Contains(raw, "Reply-To: ada@example.test") {
		t.Errorf("the encoded message lost the reply address:\n%s", raw)
	}
}

func TestContactFormRejectsAnIncompleteSubmission(t *testing.T) {
	for name, form := range map[string]url.Values{
		"no address": {"name": {"Ada"}, "message": {"hello"}},
		"no name":    {"email": {"ada@example.test"}, "message": {"hello"}},
		"no message": {"name": {"Ada"}, "email": {"ada@example.test"}},
		"blank name": {"name": {"   "}, "email": {"ada@example.test"}, "message": {"hello"}},
	} {
		t.Run(name, func(t *testing.T) {
			contact := newContactApp(t, mail.NewMemory())

			response := contact.submit(form)
			if response.StatusCode >= 500 {
				t.Errorf("status = %d, a visitor error is not a server error", response.StatusCode)
			}
			if contact.sent.Count() != 0 {
				t.Errorf("%d messages were sent for an incomplete form", contact.sent.Count())
			}
		})
	}
}

func TestContactFormRefusesAnInjectedHeader(t *testing.T) {
	for name, value := range map[string]string{
		"crlf in the name": "Ada\r\nBcc: attacker@evil.test",
		"lf in the name":   "Ada\nBcc: attacker@evil.test",
	} {
		t.Run(name, func(t *testing.T) {
			contact := newContactApp(t, mail.NewMemory())

			form := validEnquiry()
			form.Set("name", value)

			response := contact.submit(form)
			if response.StatusCode >= 500 {
				t.Errorf("status = %d, an injection attempt is not a server error", response.StatusCode)
			}
			if contact.sent.Count() != 0 {
				t.Fatalf("an injected header was sent: %+v", contact.sent.Sent())
			}
		})
	}
}

func TestContactFormReportsAFailureWithoutA500(t *testing.T) {
	contact := newContactApp(t, senderFunc(func(context.Context, mail.Message) error {
		return errors.New("the relay refused the connection")
	}))

	response := contact.submit(validEnquiry())
	if response.StatusCode >= 500 {
		t.Errorf("status = %d, the visitor should see a flash rather than a 500", response.StatusCode)
	}
	if !strings.Contains(contact.log.String(), "the relay refused the connection") {
		t.Errorf("the failure was not logged: %q", contact.log.String())
	}
}

type countingBackend struct {
	inner  mail.Sender
	sent   atomic.Int64
	failed atomic.Int64
}

func (c *countingBackend) Send(ctx context.Context, message mail.Message) error {
	if err := c.inner.Send(ctx, message); err != nil {
		c.failed.Add(1)
		return err
	}
	c.sent.Add(1)
	return nil
}

func TestContactFormThroughACustomDecoratorBackend(t *testing.T) {
	inner := mail.NewMemory()
	outbox := &countingBackend{inner: inner}

	contact := newContactApp(t, outbox)

	if _, err := contact.app.Settings.Email.Open(); err != nil {
		t.Fatalf("Open should hand back the decorator: %v", err)
	}

	contact.submit(validEnquiry())
	contact.submit(url.Values{"name": {"Ada"}})

	if outbox.sent.Load() != 1 {
		t.Errorf("the decorator counted %d sends, want 1", outbox.sent.Load())
	}
	if outbox.failed.Load() != 0 {
		t.Errorf("the decorator counted %d failures, want 0", outbox.failed.Load())
	}
	if inner.Count() != 1 {
		t.Errorf("the inner backend holds %d messages, want 1", inner.Count())
	}

	failing := &countingBackend{inner: senderFunc(func(context.Context, mail.Message) error {
		return errors.New("nope")
	})}
	broken := newContactApp(t, failing)
	broken.submit(validEnquiry())

	if failing.failed.Load() != 1 {
		t.Errorf("the decorator counted %d failures, want 1", failing.failed.Load())
	}
}

func TestDocumentedPasswordResetFlowEmailsAWorkingLink(t *testing.T) {
	outbox := mail.NewMemory()

	a := newTestApp(t, func(s *settings.Settings) {
		s.Auth.ResetTokens = true
		s.Auth.ResetTokenLifetime = time.Hour
		s.Email = settings.Email{From: "shop@example.test", Sender: outbox}
	})

	if _, err := a.Auth.CreateUser(auth.NewUser{
		Username: "ada",
		Email:    "ada@example.test",
		Password: "her-first-secret",
	}); err != nil {
		t.Fatal(err)
	}

	sender, err := a.Settings.Email.Open()
	if err != nil {
		t.Fatal(err)
	}
	resetLog := &strings.Builder{}
	a.Post("/reset/request", requestReset(a, sender, resetLog))

	client := newClient(t, a.Handler())
	for _, address := range []string{"ada@example.test", "nobody@example.test"} {
		response := client.do(http.MethodPost, "/reset/request",
			url.Values{"email": {address}}).Result()
		if response.StatusCode >= 500 {
			t.Fatalf("status = %d for %s", response.StatusCode, address)
		}
	}

	if resetLog.Len() != 0 {
		t.Fatalf("the handler logged a failure: %s", resetLog.String())
	}
	if outbox.Count() != 1 {
		t.Fatalf("%d messages sent; an unknown address must not send one", outbox.Count())
	}

	message, _ := outbox.Last()
	if message.To[0] != "ada@example.test" {
		t.Errorf("To = %v", message.To)
	}
	if message.From != "shop@example.test" {
		t.Errorf("From = %q, the configured sender should fill it in", message.From)
	}

	_, query, found := strings.Cut(message.Text, "token=")
	if !found {
		t.Fatalf("no token in the email:\n%s", message.Text)
	}
	token, err := url.QueryUnescape(strings.TrimSpace(query))
	if err != nil {
		t.Fatalf("the token is not url-safe: %v", err)
	}

	if _, err := a.Auth.CheckResetToken(token); err != nil {
		t.Fatalf("the emailed token does not work: %v", err)
	}
	if _, err := a.Auth.UseResetToken(token, auth.PasswordChange{
		New: "a-second-secret", Confirm: "a-second-secret",
	}); err != nil {
		t.Fatalf("consuming the emailed token: %v", err)
	}
	if _, err := a.Auth.CheckResetToken(token); err == nil {
		t.Error("the token should be single use")
	}

	if strings.Contains(message.Text, auth.TokenDigest(token)) {
		t.Error("the email should carry the token, never its digest")
	}
}

func requestReset(a *app.App, sender mail.Sender, resetLog *strings.Builder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		address := r.PostForm.Get("email")

		user, err := a.Auth.Users().ByEmail(address)
		if err == nil {
			plain, err := a.Auth.CreateResetToken(user.ID)
			if err != nil {
				resetLog.WriteString(err.Error())
			} else {
				link := "https://shop.test/reset?token=" + url.QueryEscape(plain)
				err = sender.Send(r.Context(), mail.Message{
					From:    a.Settings.Email.From,
					To:      []string{address},
					Subject: "Reset your password",
					Text:    "Open this link within the hour:\n\n" + link + "\n",
				})
				if err != nil {
					resetLog.WriteString(err.Error())
				}
			}
		}

		view.Flash(r, "success", "If that address has an account, a link is on its way.")
		view.Redirect(w, r, "/accounts/login")
	}
}

func TestATemplateCanRenderAnEmailBody(t *testing.T) {
	outbox := mail.NewMemory()
	a := newTestApp(t, func(s *settings.Settings) {
		s.Email = settings.Email{From: "shop@example.test", Sender: outbox}
	})

	buf, err := a.Templates.RenderToBuffer("pages/hello.html", view.Data{"Name": "Receipt"})
	if err != nil {
		t.Fatalf("RenderToBuffer: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("the rendered body is empty")
	}

	message := mail.Message{
		From:    a.Settings.Email.From,
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		HTML:    buf.String(),
	}
	if err := outbox.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent, _ := outbox.Last()
	if !strings.Contains(sent.HTML, "Receipt") {
		t.Errorf("the rendered body did not reach the message: %q", sent.HTML)
	}
}
