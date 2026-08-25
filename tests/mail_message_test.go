package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/lib/mail"
)

func validMessage() mail.Message {
	return mail.Message{
		From:    "Shop <shop@example.test>",
		To:      []string{"ada@example.test"},
		Subject: "Your order",
		Text:    "Thank you.",
	}
}

func TestMessageValidatesAGoodMessage(t *testing.T) {
	if err := validMessage().Validate(); err != nil {
		t.Errorf("a well formed message should validate: %v", err)
	}
}

func TestMessageRejectsEachFailureWithItsOwnError(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*mail.Message)
		want   error
	}{
		"no from": {
			mutate: func(m *mail.Message) { m.From = "" },
			want:   mail.ErrNoSender,
		},
		"unparseable from": {
			mutate: func(m *mail.Message) { m.From = "not an address" },
			want:   mail.ErrBadAddress,
		},
		"no recipients": {
			mutate: func(m *mail.Message) { m.To = nil },
			want:   mail.ErrNoRecipient,
		},
		"unparseable recipient": {
			mutate: func(m *mail.Message) { m.To = []string{"@@@"} },
			want:   mail.ErrBadAddress,
		},
		"unparseable reply-to": {
			mutate: func(m *mail.Message) { m.ReplyTo = "nope" },
			want:   mail.ErrBadAddress,
		},
		"no body": {
			mutate: func(m *mail.Message) { m.Text, m.HTML = "", "" },
			want:   mail.ErrNoBody,
		},
		"whitespace body": {
			mutate: func(m *mail.Message) { m.Text, m.HTML = "   \n\t ", "" },
			want:   mail.ErrNoBody,
		},
	} {
		t.Run(name, func(t *testing.T) {
			message := validMessage()
			tc.mutate(&message)

			err := message.Validate()
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMessageRejectsHeaderInjection(t *testing.T) {
	payloads := map[string]string{
		"crlf":        "Order\r\nBcc: attacker@evil.test",
		"bare lf":     "Order\nBcc: attacker@evil.test",
		"bare cr":     "Order\rBcc: attacker@evil.test",
		"folded":      "Order\r\n Bcc: attacker@evil.test",
		"trailing lf": "Order\n",
	}

	for name, payload := range payloads {
		t.Run("subject "+name, func(t *testing.T) {
			message := validMessage()
			message.Subject = payload
			if err := message.Validate(); !errors.Is(err, mail.ErrHeaderInjection) {
				t.Errorf("error = %v, want ErrHeaderInjection", err)
			}
		})

		t.Run("from "+name, func(t *testing.T) {
			message := validMessage()
			message.From = "shop@example.test" + payload
			err := message.Validate()
			if err == nil {
				t.Fatal("a From carrying a line break must be refused")
			}
			if !errors.Is(err, mail.ErrHeaderInjection) && !errors.Is(err, mail.ErrBadAddress) {
				t.Errorf("error = %v", err)
			}
		})

		t.Run("recipient "+name, func(t *testing.T) {
			message := validMessage()
			message.To = []string{"ada@example.test" + payload}
			err := message.Validate()
			if err == nil {
				t.Fatal("a recipient carrying a line break must be refused")
			}
			if !errors.Is(err, mail.ErrHeaderInjection) && !errors.Is(err, mail.ErrBadAddress) {
				t.Errorf("error = %v", err)
			}
		})

		t.Run("custom header "+name, func(t *testing.T) {
			message := validMessage().WithHeader("X-Campaign", payload)
			if err := message.Validate(); !errors.Is(err, mail.ErrHeaderInjection) {
				t.Errorf("error = %v, want ErrHeaderInjection", err)
			}
		})
	}
}

func TestMessageRefusesAReservedCustomHeader(t *testing.T) {
	for _, name := range []string{"Bcc", "bcc", "From", "Content-Type", "Message-ID"} {
		message := validMessage().WithHeader(name, "smuggled@evil.test")
		err := message.Validate()
		if err == nil {
			t.Errorf("%q should not be settable as a custom header", name)
			continue
		}
		if !strings.Contains(err.Error(), "rather than as a custom header") {
			t.Errorf("error for %q = %v", name, err)
		}
	}
}

func TestMessageRecipientsMergesAndDeduplicates(t *testing.T) {
	message := mail.Message{
		From:    "shop@example.test",
		To:      []string{"Ada <ada@example.test>", "bob@example.test"},
		CC:      []string{"ADA@example.test", "carol@example.test"},
		BCC:     []string{"dave@example.test", "bob@Example.test"},
		Subject: "Hello",
		Text:    "Hi",
	}

	got := message.Recipients()
	want := []string{"ada@example.test", "bob@example.test", "carol@example.test", "dave@example.test"}

	if len(got) != len(want) {
		t.Fatalf("Recipients = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMessageAddressShapes(t *testing.T) {
	for _, value := range []string{
		"ada@example.test",
		"Ada Lovelace <ada@example.test>",
		`"Lovelace, Ada" <ada@example.test>`,
		"Ada Lovelace <ADA@Example.TEST>",
		"  ada@example.test  ",
	} {
		if _, err := mail.ParseAddress(value); err != nil {
			t.Errorf("ParseAddress(%q) = %v", value, err)
		}
	}

	for _, value := range []string{"", "   ", "not an address", "@example.test", "ada@", "a@b@c"} {
		if _, err := mail.ParseAddress(value); err == nil {
			t.Errorf("ParseAddress(%q) should fail", value)
		}
	}
}

func TestFormatAddressEncodesADisplayName(t *testing.T) {
	got := mail.FormatAddress("Ada Lovelace", "ada@example.test")
	if got != `"Ada Lovelace" <ada@example.test>` {
		t.Errorf("FormatAddress = %q", got)
	}

	encoded := mail.FormatAddress("Ada Lovelacé", "ada@example.test")
	if !strings.Contains(encoded, "=?utf-8?") {
		t.Errorf("a non-ASCII display name should be encoded, got %q", encoded)
	}
	if strings.Contains(encoded, "é") {
		t.Errorf("raw non-ASCII must not reach a header: %q", encoded)
	}
}

func TestParseAddressListRefusesAnAbsurdNumberOfRecipients(t *testing.T) {
	many := make([]string, 500)
	for i := range many {
		many[i] = "person@example.test"
	}
	if _, err := mail.ParseAddressList(many); !errors.Is(err, mail.ErrBadAddress) {
		t.Errorf("error = %v, want ErrBadAddress", err)
	}
}

func TestMessageRefusesAnAbsurdSubject(t *testing.T) {
	message := validMessage()
	message.Subject = strings.Repeat("a", 1<<20)

	if err := message.Validate(); err == nil {
		t.Error("a one megabyte subject should be refused")
	}
}

func TestMessageBuildersDoNotMutateTheOriginal(t *testing.T) {
	original := validMessage().WithHeader("X-One", "1")

	extended := original.WithHeader("X-Two", "2").
		Attach(mail.NewAttachment("a.txt", "text/plain", []byte("hello")))

	if len(original.Headers) != 1 {
		t.Errorf("WithHeader mutated the original: %v", original.Headers)
	}
	if len(original.Attachments) != 0 {
		t.Errorf("Attach mutated the original: %v", original.Attachments)
	}
	if len(extended.Headers) != 2 || len(extended.Attachments) != 1 {
		t.Errorf("the copy is missing what was added: %+v", extended)
	}
}

func TestAttachmentTypeAndName(t *testing.T) {
	if got := mail.NewAttachment("notes.txt", "", []byte("hi")).ContentType(); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("ContentType = %q, want it sniffed from the extension", got)
	}
	if got := mail.NewAttachment("x.bin", "", []byte{0x00, 0x01}).ContentType(); got == "" {
		t.Error("an unknown extension should still produce a type")
	}
	if got := mail.NewAttachment("report.pdf", "application/pdf", nil).ContentType(); got != "application/pdf" {
		t.Errorf("a declared type should win, got %q", got)
	}

	for name, want := range map[string]string{
		"../../etc/passwd": "passwd",
		"/absolute/x.txt":  "x.txt",
		"":                 "attachment",
		"..":               "attachment",
		"quote\"name.txt":  "quotename.txt",
	} {
		if got := mail.NewAttachment(name, "text/plain", []byte("x")).Name(); got != want {
			t.Errorf("Name(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestAttachmentValidation(t *testing.T) {
	message := validMessage()
	message.Attachments = []mail.Attachment{mail.NewAttachment("empty.txt", "text/plain", nil)}
	if err := message.Validate(); err == nil {
		t.Error("an empty attachment should be refused")
	}

	message.Attachments = []mail.Attachment{
		mail.NewAttachment("ok.txt", "text/plain\r\nBcc: evil@example.test", []byte("x")),
	}
	if err := message.Validate(); !errors.Is(err, mail.ErrHeaderInjection) {
		t.Errorf("error = %v, want ErrHeaderInjection", err)
	}
}
