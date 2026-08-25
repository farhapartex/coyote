package mail

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const fallbackDomain = "localhost"

type Encoder struct {
	Now      func() time.Time
	Boundary func() string
	Domain   string
}

func NewEncoder() Encoder {
	return Encoder{Now: time.Now, Boundary: randomToken, Domain: ""}
}

func Encode(m Message) ([]byte, error) {
	return NewEncoder().Encode(m)
}

func (e Encoder) Encode(m Message) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	content, err := e.content(m)
	if err != nil {
		return nil, err
	}
	if len(m.Attachments) > 0 {
		content = e.mixed(content, m.Attachments)
	}

	header, err := e.header(m, content)
	if err != nil {
		return nil, err
	}
	return []byte(header + content.body), nil
}

func (e Encoder) header(m Message, content part) (string, error) {
	from, err := addressHeader(m.From)
	if err != nil {
		return "", err
	}
	to, err := addressListHeader(m.To)
	if err != nil {
		return "", err
	}
	cc, err := addressListHeader(m.CC)
	if err != nil {
		return "", err
	}
	replyTo, err := addressHeader(m.ReplyTo)
	if err != nil {
		return "", err
	}

	var w headerWriter
	w.set("From", from)
	w.set("To", to)
	w.set("Cc", cc)
	w.set("Reply-To", replyTo)
	w.set("Subject", encodeWord(m.Subject))
	w.set("Date", e.date(m).Format(time.RFC1123Z))
	w.set("Message-ID", e.messageID(m))
	w.set("MIME-Version", "1.0")
	w.set("Content-Type", content.contentType)
	w.set("Content-Transfer-Encoding", content.encoding)
	for _, name := range sortedHeaderNames(m.Headers) {
		w.set(name, encodeWord(m.Headers[name]))
	}
	w.blank()

	return w.String(), nil
}

func (e Encoder) content(m Message) (part, error) {
	hasText := strings.TrimSpace(m.Text) != ""
	hasHTML := strings.TrimSpace(m.HTML) != ""

	switch {
	case hasText && hasHTML:
		return e.alternative(m)
	case hasHTML:
		return textPart(htmlType, m.HTML)
	default:
		return textPart(textType, m.Text)
	}
}

func (e Encoder) alternative(m Message) (part, error) {
	text, err := textPart(textType, m.Text)
	if err != nil {
		return part{}, err
	}
	html, err := textPart(htmlType, m.HTML)
	if err != nil {
		return part{}, err
	}

	boundary := e.boundary()
	var body strings.Builder
	for _, inner := range []part{text, html} {
		boundaryOpen(&body, boundary)
		inner.write(&body)
	}
	boundaryClose(&body, boundary)

	return part{
		contentType: fmt.Sprintf("multipart/alternative; boundary=%q", boundary),
		body:        body.String(),
	}, nil
}

func (e Encoder) mixed(content part, attachments []Attachment) part {
	boundary := e.boundary()

	var body strings.Builder
	boundaryOpen(&body, boundary)
	content.write(&body)
	for _, attachment := range attachments {
		boundaryOpen(&body, boundary)
		attachmentPart(attachment).write(&body)
	}
	boundaryClose(&body, boundary)

	return part{
		contentType: fmt.Sprintf("multipart/mixed; boundary=%q", boundary),
		body:        body.String(),
	}
}

func (e Encoder) date(m Message) time.Time {
	if !m.Date.IsZero() {
		return m.Date
	}
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e Encoder) boundary() string {
	if e.Boundary != nil {
		return e.Boundary()
	}
	return randomToken()
}

func (e Encoder) messageID(m Message) string {
	if trimmed := strings.TrimSpace(m.MessageID); trimmed != "" {
		if strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") {
			return trimmed
		}
		return "<" + trimmed + ">"
	}
	return fmt.Sprintf("<%s@%s>", randomToken(), e.domain(m))
}

func (e Encoder) domain(m Message) string {
	if trimmed := strings.TrimSpace(e.Domain); trimmed != "" {
		return trimmed
	}
	if parsed, err := ParseAddress(m.From); err == nil {
		if _, domain, found := strings.Cut(parsed.Address, "@"); found && domain != "" {
			return domain
		}
	}
	return fallbackDomain
}

func randomToken() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
