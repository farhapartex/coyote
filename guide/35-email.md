# 35. Email

[← Back to contents](README.md)

Email in Coyote is a sealed block: a one-method interface, a message type, four backends and a
configuration struct. The framework sends nothing on your behalf. There is no `sendmail` command, no
hook into forms, nothing on the boot path. You construct a backend when you want one and call it where
you want to.

That is deliberate. Delivery policy — synchronous or queued, retried or not, on this request or a
worker — depends on your application, and a framework that guesses gets it wrong.

## The interface

```go
type Sender interface {
	Send(ctx context.Context, message Message) error
}
```

That is the whole contract. `lib/mail` imports nothing else from Coyote, so you can use it from a
handler, a goroutine, a cron job, a CLI command or a test.

## A message

```go
import "github.com/farhapartex/coyote/lib/mail"

message := mail.Message{
	From:    "Shop <shop@example.test>",
	To:      []string{"ada@example.test"},
	Subject: "Your order",
	Text:    "Thank you for your order.",
	HTML:    "<p>Thank you for your order.</p>",
}
```

| Field | |
| --- | --- |
| `From` | required; a bare address or `Name <address>` |
| `To`, `CC`, `BCC` | at least one recipient across the three |
| `ReplyTo` | optional |
| `Subject` | optional, capped at 4096 characters |
| `Text`, `HTML` | at least one must be non-blank |
| `Headers` | extra headers; reserved names are refused |
| `Attachments` | see below |
| `Date`, `MessageID` | filled in for you when left zero |

The structure of the encoded message follows what you set:

| Message holds | Content type |
| --- | --- |
| text only | `text/plain; charset=utf-8`, quoted-printable |
| HTML only | `text/html; charset=utf-8`, quoted-printable |
| both | `multipart/alternative`, text part first |
| any attachments | `multipart/mixed` wrapping the above, attachments base64 |

Non-ASCII subjects and display names are encoded with RFC 2047, so `Réception — 商品` arrives intact
without raw bytes reaching a header.

Builders return a copy rather than mutating:

```go
message = message.
	WithHeader("X-Campaign", "spring").
	Attach(mail.NewAttachment("receipt.txt", "text/plain", []byte("total 12.00\n")))
```

`Recipients()` gives the envelope: `To`, `CC` and `BCC` merged, deduplicated case-insensitively, in
that order.

## Attachments

```go
mail.NewAttachment("receipt.pdf", "application/pdf", pdfBytes)
mail.FileAttachment("/var/lib/app/receipt.pdf")   // reads the file, keeps the base name
```

An empty declared type is sniffed from the extension, then from the content. Filenames are reduced to
their base name, so an attachment called `../../etc/passwd` is sent as `passwd`.

## The four built-in backends

```go
mail.NewConsole(os.Stdout)   // prints the encoded message
mail.NewFile("mail")         // writes <unix-nano>-<random>.eml at 0600
mail.NewMemory()             // records messages, for tests
mail.NewSMTP(mail.SMTPOptions{Host: "smtp.example.test", TLS: mail.TLSStartTLS})
```

All four are safe for concurrent use. The file backend writes through a temporary file and a rename,
so a reader never sees half a message. The SMTP backend dials per send, so there is no shared
connection to guard.

`MemorySender` is the one you want in tests:

```go
outbox := mail.NewMemory()
// ... exercise your code with outbox as the Sender ...
if outbox.Count() != 1 {
	t.Fatalf("sent %d messages", outbox.Count())
}
message, _ := outbox.Last()
```

`Sent()` and `Last()` return deep copies, so a test cannot accidentally mutate the record. `Raw()`
gives the encoded bytes when you want to assert on the wire format instead.

## Configuration

```go
s.Email = settings.Email{
	Backend: settings.EmailToSMTP,
	Host:    settings.Env("SMTP_HOST", "127.0.0.1"),
	Port:    settings.EnvInt("SMTP_PORT", 587),
	Username: settings.Env("SMTP_USER", ""),
	Password: settings.Env("SMTP_PASSWORD", ""),
	TLS:     mail.TLSStartTLS,
	From:    "hello@shop.test",
}
```

| Field | |
| --- | --- |
| `Backend` | `"smtp"`, `"console"`, `"file"`, `"memory"`, or empty |
| `Host`, `Port` | SMTP only; the port defaults to 587, 465 or 25 by TLS mode |
| `Username`, `Password` | SMTP only; refused with `TLS: "none"` |
| `TLS` | `"none"`, `"starttls"` (default) or `"tls"` |
| `From` | the SMTP backend's default sender; see the note under password reset |
| `Dir` | file backend only; relative paths resolve against `BaseDir` |
| `Timeout` | per send, default 10s |
| `LocalName` | the name given in `EHLO`, default `localhost` |
| `Sender` | your own backend — see below |

**An empty `Backend` is valid.** A project that never sends email configures nothing, and
`Open()` returns `mail.ErrNotConfigured` if something asks for a backend anyway.

Building the backend is your call, not the framework's:

```go
sender, err := a.Settings.Email.Open()
if err != nil {
	log.Fatal(err)
}
```

`Open()` is a factory, so call it once at startup and keep the result. Nothing in Coyote calls it for
you.

`Redacted()` masks the password for logging, and a configuration error never quotes it.

## Sending

Synchronously, reporting the failure to the visitor:

```go
ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
defer cancel()

if err := sender.Send(ctx, message); err != nil {
	view.Flash(r, "error", "We could not send your message.")
	view.Redirect(w, r, "/contact")
	return
}
```

Or handed off, when the visitor should not wait:

```go
go func() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := sender.Send(ctx, message); err != nil {
		log.Printf("delivery to %v failed: %v", message.To, err)
	}
}()
```

Note the `context.Background()`. The request context is cancelled the moment the response is written,
so passing `r.Context()` into a goroutine that outlives the request cancels the send. A bare goroutine
also drops the message if the process stops; a real deployment wants a queue, and `Sender` is exactly
the interface a queue worker consumes.

## Rendering a body from a template

A body is a string, so there is no email-specific template machinery. The engine you already have
renders one:

```go
buf, err := a.Templates.RenderToBuffer("email/receipt.html", view.Data{"Order": order})
if err != nil {
	return err
}
message.HTML = buf.String()
```

`RenderToBuffer` gives you the rendered page instead of writing it to a `ResponseWriter`, which is all
an email body needs.

## Your own backend

This is the point of the section. `Sender` has one method, so a transactional API, a queue writer or a
test double is a few lines:

```go
type sesSender struct {
	client *ses.Client
	from   string
}

func (s sesSender) Send(ctx context.Context, message mail.Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	raw, err := mail.Encode(message)
	if err != nil {
		return err
	}
	_, err = s.client.SendRawEmail(ctx, &ses.SendRawEmailInput{
		Source:       &s.from,
		Destinations: message.Recipients(),
		RawMessage:   &ses.RawMessage{Data: raw},
	})
	return err
}
```

`mail.Encode` gives you RFC 5322 bytes, and `Recipients()` gives the envelope, so an API that takes a
raw message needs no MIME work from you.

Then hand it to the settings, with no `Backend` set:

```go
s.Email = settings.Email{From: "hello@shop.test", Sender: sesSender{client, "hello@shop.test"}}
```

`Open()` returns your backend untouched. Setting both `Sender` and `Backend` is a configuration error —
choose one. The rest of the `Email` block is still validated, so a malformed `From` is caught whether
the backend is ours or yours.

The contract a backend should honour:

1. **Validate first.** `message.Validate()` before anything else, so a bad message never reaches the
   network.
2. **Respect the context.** Return `ctx.Err()` when it is already done, and pass it to whatever you
   call.
3. **Be safe for concurrent use.** Callers will send from goroutines.
4. **Never put `BCC` in the body.** It belongs in the envelope only.
5. **Return a wrapped sentinel** where one fits — `mail.ErrSendFailed`, `mail.ErrTooLarge` — so callers
   can use `errors.Is`.
6. **Send nothing on failure.** Partial writes are worse than a clean error.

Because a backend is just a `Sender`, decorating one is the same shape. This counts what goes out:

```go
type countingSender struct {
	inner mail.Sender
	sent  atomic.Int64
}

func (c *countingSender) Send(ctx context.Context, message mail.Message) error {
	if err := c.inner.Send(ctx, message); err != nil {
		return err
	}
	c.sent.Add(1)
	return nil
}
```

The same shape gives you logging, metrics, retries, or a rate limit, without the framework offering an
opinion about any of them.

## Security

Three things this package refuses to do, each with a test behind it.

**Header injection.** A carriage return or newline anywhere in `From`, a recipient, `Subject`,
`MessageID`, a custom header name or value, or an attachment filename or type is rejected with
`mail.ErrHeaderInjection`. This is the attack that turns a contact form into an open relay:

```
Subject: Hello\r\nBcc: attacker@evil.test
```

Validation happens before any dial or write, so a rejected message never touches the network. Reserved
headers — `Bcc`, `From`, `Content-Type` and the rest — cannot be smuggled in through `Headers` either;
set the fields instead.

**Credentials in clear.** SMTP credentials with `TLS: "none"` are refused at three layers: settings
validation, the sender before it authenticates, and the authentication mechanism itself. Coyote
implements PLAIN and LOGIN itself rather than using `smtp.PlainAuth`, because the standard library
permits cleartext credentials to `localhost` and `127.0.0.1` and Coyote does not. `TLS: "starttls"`
fails when the server does not advertise STARTTLS rather than continuing unencrypted.

**BCC disclosure.** `BCC` addresses go into the SMTP envelope as `RCPT TO` and are never written into
the message, so recipients cannot see the blind copy list.

## Password reset, which is yours

The framework supplies the token primitives and stops there, because the delivery half is yours. In
`core/auth`:

```go
plain, err := a.Auth.CreateResetToken(ctx, user.ID) // stores only a digest
user, err := a.Auth.CheckResetToken(ctx, plain)     // validates without consuming
user, err := a.Auth.UseResetToken(ctx, plain, auth.PasswordChange{New: pw, Confirm: pw})
```

The whole flow is about thirty lines of application code:

```go
func requestReset(a *app.App, sender mail.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		address := r.PostForm.Get("email")

		user, err := a.Auth.Users().ByEmail(address)
		if err == nil {
			plain, err := a.Auth.CreateResetToken(r.Context(), user.ID)
			if err != nil {
				log.Printf("reset token for %s: %v", user.ID, err)
			} else {
				link := "https://shop.test/reset?token=" + url.QueryEscape(plain)
				err = sender.Send(r.Context(), mail.Message{
					From:    a.Settings.Email.From,
					To:      []string{address},
					Subject: "Reset your password",
					Text:    "Open this link within the hour:\n\n" + link + "\n",
				})
				if err != nil {
					log.Printf("reset email to %s: %v", address, err)
				}
			}
		}

		view.Flash(r, "success", "If that address has an account, a link is on its way.")
		view.Redirect(w, r, "/accounts/login")
	}
}
```

Three things worth copying. The flash is the same whether or not the address exists, so the page is not
an account enumeration oracle. The token is emailed but never logged — only its digest is stored, so a
database leak does not hand over working reset links. And the send error is logged rather than
discarded: `Send` validates the message, so a discarded error is a message that silently never went.

Note the explicit `From`. `Email.From` is read by the SMTP backend, which fills it in when a message
leaves it blank; the console, file and memory backends do not, and `Send` returns `mail.ErrNoSender`
for a message with no sender. Set it on the message, or wrap the backend once at startup:

```go
sender = mail.WithDefaultFrom(sender, a.Settings.Email.From)
```

See [Authentication](14-authentication.md) for the token store, lifetimes and `PasswordChange`.

## What is not here

No queue, no retry policy, no scheduled sending, no template-to-email helper, no bounce handling, no
open or click tracking. Each one is a policy decision, and `Sender` is the seam where you supply your
own.

## Next

[Architecture →](27-architecture.md)
