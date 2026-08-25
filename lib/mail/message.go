package mail

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

const maxSubjectLength = 4096

var reservedHeaders = map[string]bool{
	"bcc":                       true,
	"from":                      true,
	"to":                        true,
	"cc":                        true,
	"subject":                   true,
	"reply-to":                  true,
	"date":                      true,
	"message-id":                true,
	"mime-version":              true,
	"content-type":              true,
	"content-transfer-encoding": true,
}

type Message struct {
	From        string
	To          []string
	CC          []string
	BCC         []string
	ReplyTo     string
	Subject     string
	Text        string
	HTML        string
	Headers     map[string]string
	Attachments []Attachment
	Date        time.Time
	MessageID   string
}

func (m Message) Recipients() []string {
	out := make([]string, 0, len(m.To)+len(m.CC)+len(m.BCC))
	seen := make(map[string]bool, cap(out))

	for _, group := range [][]string{m.To, m.CC, m.BCC} {
		for _, entry := range group {
			parsed, err := ParseAddress(entry)
			if err != nil {
				continue
			}
			key := strings.ToLower(parsed.Address)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, parsed.Address)
		}
	}
	return out
}

func (m Message) HasBody() bool {
	return strings.TrimSpace(m.Text) != "" || strings.TrimSpace(m.HTML) != ""
}

func (m Message) WithHeader(name, value string) Message {
	copied := m
	copied.Headers = maps.Clone(m.Headers)
	if copied.Headers == nil {
		copied.Headers = map[string]string{}
	}
	copied.Headers[name] = value
	return copied
}

func (m Message) Attach(attachments ...Attachment) Message {
	copied := m
	copied.Attachments = slices.Concat(m.Attachments, attachments)
	return copied
}

func (m Message) Validate() error {
	if strings.TrimSpace(m.From) == "" {
		return ErrNoSender
	}
	if _, err := ParseAddress(m.From); err != nil {
		return err
	}
	if strings.TrimSpace(m.ReplyTo) != "" {
		if _, err := ParseAddress(m.ReplyTo); err != nil {
			return err
		}
	}

	if len(m.To)+len(m.CC)+len(m.BCC) == 0 {
		return ErrNoRecipient
	}
	for _, group := range [][]string{m.To, m.CC, m.BCC} {
		if _, err := ParseAddressList(group); err != nil {
			return err
		}
	}
	if len(m.Recipients()) == 0 {
		return ErrNoRecipient
	}

	if containsBreak(m.Subject) {
		return fmt.Errorf("%w: subject", ErrHeaderInjection)
	}
	if len(m.Subject) > maxSubjectLength {
		return fmt.Errorf("coyote/mail: the subject is longer than %d characters", maxSubjectLength)
	}
	if containsBreak(m.MessageID) {
		return fmt.Errorf("%w: message id", ErrHeaderInjection)
	}
	if !m.HasBody() {
		return ErrNoBody
	}

	for name, value := range m.Headers {
		if err := checkHeader(name, value); err != nil {
			return err
		}
	}
	for _, attachment := range m.Attachments {
		if err := attachment.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func checkHeader(name, value string) error {
	if containsBreak(name) || containsBreak(value) {
		return fmt.Errorf("%w: header %q", ErrHeaderInjection, name)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("coyote/mail: a header has no name")
	}
	if reservedHeaders[strings.ToLower(strings.TrimSpace(name))] {
		return fmt.Errorf("coyote/mail: set %s on the message rather than as a custom header", name)
	}
	return nil
}
