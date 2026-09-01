package tests

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/lib/mail"
)

func fixedEncoder() mail.Encoder {
	counter := 0
	return mail.Encoder{
		Now: func() time.Time {
			return time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
		},
		Boundary: func() string {
			counter++
			return fmt.Sprintf("BOUNDARY%d", counter)
		},
		Domain: "example.test",
	}
}

func encode(t *testing.T, message mail.Message) (*netmail.Message, string) {
	t.Helper()

	raw, err := fixedEncoder().Encode(message)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	parsed, err := netmail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("the encoded message does not parse: %v\n%s", err, raw)
	}
	return parsed, string(raw)
}

func partTree(t *testing.T, parsed *netmail.Message) map[string]string {
	t.Helper()

	found := map[string]string{}
	collectParts(t, parsed.Header.Get("Content-Type"), parsed.Body, found)
	return found
}

func collectParts(t *testing.T, contentType string, body io.Reader, found map[string]string) {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("Content-Type %q: %v", contentType, err)
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		content, err := io.ReadAll(quotedprintable.NewReader(body))
		if err != nil {
			t.Fatalf("reading a %s part: %v", mediaType, err)
		}
		found[mediaType] = string(content)
		return
	}

	reader := multipart.NewReader(body, params["boundary"])
	for {
		next, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("walking %s: %v", mediaType, err)
		}

		inner := next.Header.Get("Content-Type")
		if next.Header.Get("Content-Transfer-Encoding") == "base64" {
			decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, next))
			if err != nil {
				t.Fatalf("reading an attachment: %v", err)
			}
			found["attachment:"+next.FileName()] = string(decoded)
			continue
		}
		collectParts(t, inner, next, found)
	}
}

func TestEncodeTextOnly(t *testing.T) {
	parsed, raw := encode(t, mail.Message{
		From:    "Shop <shop@example.test>",
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		Text:    "Thank you.",
	})

	if got := parsed.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(raw, "Content-Transfer-Encoding: quoted-printable") {
		t.Error("a text body should be quoted-printable")
	}
	if got := partTree(t, parsed)["text/plain"]; got != "Thank you." {
		t.Errorf("body = %q", got)
	}
	if got := parsed.Header.Get("Date"); got != "Wed, 04 Mar 2026 09:30:00 +0000" {
		t.Errorf("Date = %q", got)
	}
	if _, err := netmail.ParseDate(parsed.Header.Get("Date")); err != nil {
		t.Errorf("Date does not parse: %v", err)
	}
	if got := parsed.Header.Get("MIME-Version"); got != "1.0" {
		t.Errorf("MIME-Version = %q", got)
	}
	if got := parsed.Header.Get("Message-ID"); !strings.HasSuffix(got, "@example.test>") {
		t.Errorf("Message-ID = %q", got)
	}
}

func TestEncodeHTMLOnly(t *testing.T) {
	parsed, _ := encode(t, mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		HTML:    "<p>Thank you.</p>",
	})

	tree := partTree(t, parsed)
	if got := tree["text/html"]; got != "<p>Thank you.</p>" {
		t.Errorf("html = %q", got)
	}
	if _, ok := tree["text/plain"]; ok {
		t.Error("there should be no text part")
	}
}

func TestEncodeBothBodiesBecomesAlternativeWithTextFirst(t *testing.T) {
	message := mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		Text:    "Thank you.",
		HTML:    "<p>Thank you.</p>",
	}

	parsed, raw := encode(t, message)

	if got := parsed.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/alternative") {
		t.Errorf("Content-Type = %q", got)
	}

	tree := partTree(t, parsed)
	if tree["text/plain"] != "Thank you." || tree["text/html"] != "<p>Thank you.</p>" {
		t.Errorf("part tree = %v", tree)
	}

	textAt := strings.Index(raw, "text/plain")
	htmlAt := strings.Index(raw, "text/html")
	if textAt < 0 || htmlAt < 0 || textAt > htmlAt {
		t.Error("the text part must come before the HTML part")
	}
	if strings.Contains(raw, "Content-Transfer-Encoding: \r\n") {
		t.Errorf("a multipart part carries an empty transfer encoding:\n%s", raw)
	}
}

func TestEncodeWritesNoEmptyHeaders(t *testing.T) {
	_, raw := encode(t, mail.Message{
		From:        "shop@example.test",
		To:          []string{"ada@example.test"},
		Subject:     "Receipt",
		Text:        "Thank you.",
		HTML:        "<p>Thank you.</p>",
		Attachments: []mail.Attachment{mail.NewAttachment("a.txt", "text/plain", []byte("x"))},
	})

	headers, _, found := strings.Cut(raw, "\r\n\r\n")
	if !found {
		t.Fatal("the message has no header block")
	}
	for _, line := range strings.Split(headers, "\r\n") {
		name, value, isHeader := strings.Cut(line, ":")
		if isHeader && strings.TrimSpace(value) == "" {
			t.Errorf("header %q has no value", name)
		}
	}
	if strings.Contains(raw, ": \r\n") {
		t.Errorf("an empty header reached the wire:\n%s", raw)
	}
}

func TestEncodeWritesABareAddressWhenThereIsNoDisplayName(t *testing.T) {
	parsed, _ := encode(t, mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test", "Bob <bob@example.test>"},
		Subject: "Receipt",
		Text:    "Thank you.",
	})

	if got := parsed.Header.Get("From"); got != "shop@example.test" {
		t.Errorf("From = %q, want a bare address", got)
	}
	if got := parsed.Header.Get("To"); got != `ada@example.test, "Bob" <bob@example.test>` {
		t.Errorf("To = %q", got)
	}

	list, err := parsed.Header.AddressList("To")
	if err != nil || len(list) != 2 {
		t.Fatalf("AddressList = %v (%v)", list, err)
	}
}

func TestEncodeWithAttachmentBecomesMixed(t *testing.T) {
	parsed, _ := encode(t, mail.Message{
		From:        "shop@example.test",
		To:          []string{"ada@example.test"},
		Subject:     "Receipt",
		Text:        "Thank you.",
		HTML:        "<p>Thank you.</p>",
		Attachments: []mail.Attachment{mail.NewAttachment("receipt.txt", "text/plain", []byte("total 12"))},
	})

	if got := parsed.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/mixed") {
		t.Errorf("Content-Type = %q", got)
	}

	tree := partTree(t, parsed)
	for _, want := range []string{"text/plain", "text/html", "attachment:receipt.txt"} {
		if _, ok := tree[want]; !ok {
			t.Errorf("%s is missing from %v", want, tree)
		}
	}
	if got := tree["attachment:receipt.txt"]; got != "total 12" {
		t.Errorf("attachment content = %q", got)
	}
}

var bccHeader = regexp.MustCompile(`(?im)^bcc[ \t]*:`)

func TestEncodeNeverWritesBcc(t *testing.T) {
	message := mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		CC:      []string{"carol@example.test"},
		BCC:     []string{"secret@example.test", "Hidden <hidden@example.test>"},
		Subject: "Receipt",
		Text:    "Thank you.",
	}

	parsed, raw := encode(t, message)

	if strings.Contains(raw, "secret@example.test") || strings.Contains(raw, "hidden@example.test") {
		t.Fatalf("a blind copy leaked into the message:\n%s", raw)
	}
	if bccHeader.MatchString(raw) {
		t.Errorf("a Bcc header was written:\n%s", raw)
	}
	if got := parsed.Header.Get("Bcc"); got != "" {
		t.Errorf("Bcc header = %q", got)
	}
	if got := parsed.Header.Get("Cc"); !strings.Contains(got, "carol@example.test") {
		t.Errorf("Cc = %q, a visible copy should survive", got)
	}

	recipients := message.Recipients()
	if len(recipients) != 4 {
		t.Errorf("Recipients = %v, the envelope still needs the blind copies", recipients)
	}
}

func TestEncodeNonASCIISubjectRoundTrips(t *testing.T) {
	subject := "Réception de votre commande — 商品"

	parsed, raw := encode(t, mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: subject,
		Text:    "Merci.",
	})

	if strings.Contains(raw, "商品") {
		t.Error("a raw non-ASCII subject must not reach the wire")
	}

	decoded, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if decoded != subject {
		t.Errorf("subject = %q, want %q", decoded, subject)
	}
}

func TestEncodeNonASCIIDisplayNameRoundTrips(t *testing.T) {
	parsed, raw := encode(t, mail.Message{
		From:    "Boutique Café <shop@example.test>",
		To:      []string{"Ada Lovelacé <ada@example.test>"},
		Subject: "Receipt",
		Text:    "Merci.",
	})

	if strings.Contains(raw, "Café") {
		t.Error("a raw non-ASCII display name must not reach the wire")
	}

	from, err := parsed.Header.AddressList("From")
	if err != nil {
		t.Fatalf("AddressList: %v", err)
	}
	if from[0].Name != "Boutique Café" {
		t.Errorf("From name = %q", from[0].Name)
	}
}

func TestEncodeBodyEdgeCasesSurviveQuotedPrintable(t *testing.T) {
	body := "a = b\r\ntrailing spaces   \nline\rwith bare cr\n" +
		strings.Repeat("long ", 60) + "\n=3D not decoded twice\ntab\there"

	parsed, _ := encode(t, mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Edges",
		Text:    body,
	})

	got := partTree(t, parsed)["text/plain"]
	want := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	want = strings.ReplaceAll(want, "\n", "\r\n")

	if got != want {
		t.Errorf("body did not survive:\n got %q\nwant %q", got, want)
	}
}

func TestEncodeLongLinesAreWrappedWithinTheLimit(t *testing.T) {
	raw, err := fixedEncoder().Encode(mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Long",
		Text:    strings.Repeat("abcdefghij", 200),
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("a line is %d characters, over the RFC 5322 limit", len(line))
		}
	}
}

func TestEncodeLargeBinaryAttachment(t *testing.T) {
	content := make([]byte, 1<<20)
	for i := range content {
		content[i] = byte(i % 251)
	}

	message := mail.Message{
		From:        "shop@example.test",
		To:          []string{"ada@example.test"},
		Subject:     "Backup",
		Text:        "Attached.",
		Attachments: []mail.Attachment{mail.NewAttachment("blob.bin", "", content)},
	}

	parsed, raw := encode(t, message)

	for _, line := range strings.Split(raw, "\r\n") {
		if len(line) > 76 && !strings.HasPrefix(line, "Content-") && !strings.Contains(line, "@") {
			t.Fatalf("a base64 line is %d characters", len(line))
		}
	}

	got := partTree(t, parsed)["attachment:blob.bin"]
	if got != string(content) {
		t.Errorf("the attachment did not decode byte-identically: %d bytes back, want %d",
			len(got), len(content))
	}
}

func TestEncodeProducesADifferentBoundaryEveryTime(t *testing.T) {
	message := mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		Text:    "Thank you.",
		HTML:    "<p>Thank you.</p>",
	}

	first, err := mail.Encode(message)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := mail.Encode(message)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	boundaryOf := func(raw []byte) string {
		parsed, err := netmail.ReadMessage(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		_, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType: %v", err)
		}
		return params["boundary"]
	}

	if boundaryOf(first) == boundaryOf(second) {
		t.Error("two encodes must not share a boundary")
	}
	if boundaryOf(first) == "" {
		t.Error("the default encoder produced no boundary")
	}
}

func TestEncodeKeepsCustomHeadersAndRefusesReservedOnes(t *testing.T) {
	parsed, _ := encode(t, mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		Text:    "Thank you.",
	}.WithHeader("X-Campaign", "spring").WithHeader("X-Note", "café"))

	if got := parsed.Header.Get("X-Campaign"); got != "spring" {
		t.Errorf("X-Campaign = %q", got)
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("X-Note"))
	if err != nil || decoded != "café" {
		t.Errorf("X-Note = %q (%v)", decoded, err)
	}

	smuggled := mail.Message{
		From:    "shop@example.test",
		To:      []string{"ada@example.test"},
		Subject: "Receipt",
		Text:    "Thank you.",
	}.WithHeader("Bcc", "attacker@evil.test")

	if _, err := mail.Encode(smuggled); err == nil {
		t.Error("a custom Bcc header must not be encodable")
	}
}

func TestEncodeRefusesAnInvalidMessage(t *testing.T) {
	if _, err := mail.Encode(mail.Message{}); !errors.Is(err, mail.ErrNoSender) {
		t.Errorf("error = %v, want ErrNoSender", err)
	}

	injected := mail.Message{
		From:      "shop@example.test",
		To:        []string{"ada@example.test"},
		Subject:   "Receipt",
		Text:      "Thank you.",
		MessageID: "<a@b>\r\nBcc: attacker@evil.test",
	}
	if _, err := mail.Encode(injected); !errors.Is(err, mail.ErrHeaderInjection) {
		t.Errorf("error = %v, want ErrHeaderInjection", err)
	}
}

func TestEncodeHonoursAGivenDateAndMessageID(t *testing.T) {
	when := time.Date(2019, 7, 1, 12, 0, 0, 0, time.FixedZone("CET", 3600))

	parsed, _ := encode(t, mail.Message{
		From:      "shop@example.test",
		To:        []string{"ada@example.test"},
		Subject:   "Receipt",
		Text:      "Thank you.",
		Date:      when,
		MessageID: "fixed-id@example.test",
	})

	got, err := netmail.ParseDate(parsed.Header.Get("Date"))
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if !got.Equal(when) {
		t.Errorf("Date = %v, want %v", got, when)
	}
	if id := parsed.Header.Get("Message-ID"); id != "<fixed-id@example.test>" {
		t.Errorf("Message-ID = %q", id)
	}
}
