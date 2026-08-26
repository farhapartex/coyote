package main

import (
	"context"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
	"github.com/farhapartex/coyote/lib/mail"
)

func mailSender(a *app.App) (mail.Sender, error) {
	return a.Settings.Email.Open()
}

func contactPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/contact.html", view.Data{
			"Title":   i18n.T(r.Context(), "Contact us"),
			"Backend": a.Settings.Email.Location(),
		})
	}
}

func contactSend(a *app.App, sender mail.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		message, err := buildEnquiry(r, a.Settings.Email.From)
		if err != nil {
			view.Flash(r, "error", enquiryProblem(r, err))
			view.Redirect(w, r, "/contact")
			return
		}

		if r.PostForm.Get("background") == "yes" {
			deliverInBackground(sender, message)
			view.Flash(r, "success", i18n.T(r.Context(), "Thank you, we will be in touch."))
			view.Redirect(w, r, "/contact")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		if err := sender.Send(ctx, message); err != nil {
			log.Printf("contact form: sending to %v failed: %v", message.To, err)
			view.Flash(r, "error", i18n.T(r.Context(), "We could not send your message. Please try again."))
			view.Redirect(w, r, "/contact")
			return
		}

		view.Flash(r, "success", i18n.T(r.Context(), "Thank you, your message is on its way."))
		view.Redirect(w, r, "/contact")
	}
}

func buildEnquiry(r *http.Request, from string) (mail.Message, error) {
	name := strings.TrimSpace(r.PostForm.Get("name"))
	address := strings.TrimSpace(r.PostForm.Get("email"))
	body := strings.TrimSpace(r.PostForm.Get("message"))

	if name == "" || address == "" || body == "" {
		return mail.Message{}, errIncompleteEnquiry
	}
	if _, err := mail.ParseAddress(address); err != nil {
		return mail.Message{}, err
	}

	message := mail.Message{
		From:    from,
		To:      []string{from},
		ReplyTo: address,
		Subject: "Enquiry from " + name,
		Text:    fmt.Sprintf("%s <%s> wrote:\n\n%s\n", name, address, body),
		HTML: fmt.Sprintf("<p><strong>%s</strong> &lt;%s&gt; wrote:</p><p>%s</p>",
			htmltemplate.HTMLEscapeString(name), htmltemplate.HTMLEscapeString(address), htmltemplate.HTMLEscapeString(body)),
	}
	if err := message.Validate(); err != nil {
		return mail.Message{}, err
	}
	return message, nil
}

var errIncompleteEnquiry = errors.New("the enquiry is missing a name, an address or a message")

func enquiryProblem(r *http.Request, err error) string {
	switch {
	case errors.Is(err, errIncompleteEnquiry):
		return i18n.T(r.Context(), "Please fill in your name, your email address and a message.")
	case errors.Is(err, mail.ErrHeaderInjection):
		return i18n.T(r.Context(), "That message contains something we cannot send.")
	case errors.Is(err, mail.ErrBadAddress):
		return i18n.T(r.Context(), "That email address does not look right.")
	default:
		return i18n.T(r.Context(), "We could not prepare your message.")
	}
}

func deliverInBackground(sender mail.Sender, message mail.Message) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := sender.Send(ctx, message); err != nil {
			log.Printf("contact form: background delivery to %v failed: %v", message.To, err)
		}
	}()
}

type countingSender struct {
	inner  mail.Sender
	sent   atomic.Int64
	failed atomic.Int64
}

func counting(inner mail.Sender) *countingSender {
	return &countingSender{inner: inner}
}

func (c *countingSender) Send(ctx context.Context, message mail.Message) error {
	if err := c.inner.Send(ctx, message); err != nil {
		c.failed.Add(1)
		return err
	}
	c.sent.Add(1)
	return nil
}

func (c *countingSender) Counts() (int64, int64) {
	return c.sent.Load(), c.failed.Load()
}
