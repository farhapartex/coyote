package mail

import (
	"encoding/base64"
	"fmt"
	"mime/quotedprintable"
	"strings"
)

const base64LineLength = 76

type part struct {
	contentType string
	encoding    string
	disposition string
	body        string
}

func textPart(contentType, body string) (part, error) {
	encoded, err := quotedPrintable(body)
	if err != nil {
		return part{}, err
	}
	return part{
		contentType: contentType,
		encoding:    "quoted-printable",
		body:        encoded,
	}, nil
}

func attachmentPart(attachment Attachment) part {
	return part{
		contentType: attachment.ContentType(),
		encoding:    "base64",
		disposition: fmt.Sprintf("attachment; filename=%q", attachment.Name()),
		body:        base64Lines(attachment.Content),
	}
}

func (p part) write(builder *strings.Builder) {
	fmt.Fprintf(builder, "Content-Type: %s\r\n", p.contentType)
	if p.encoding != "" {
		fmt.Fprintf(builder, "Content-Transfer-Encoding: %s\r\n", p.encoding)
	}
	if p.disposition != "" {
		fmt.Fprintf(builder, "Content-Disposition: %s\r\n", p.disposition)
	}
	builder.WriteString("\r\n")
	builder.WriteString(p.body)
	if !strings.HasSuffix(p.body, "\r\n") {
		builder.WriteString("\r\n")
	}
}

func quotedPrintable(body string) (string, error) {
	var out strings.Builder
	writer := quotedprintable.NewWriter(&out)
	if _, err := writer.Write([]byte(normalizeBreaks(body))); err != nil {
		return "", fmt.Errorf("coyote/mail: encoding a body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("coyote/mail: encoding a body: %w", err)
	}
	return out.String(), nil
}

func normalizeBreaks(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}

func base64Lines(content []byte) string {
	encoded := base64.StdEncoding.EncodeToString(content)

	var out strings.Builder
	out.Grow(len(encoded) + 2*(len(encoded)/base64LineLength+1))
	for len(encoded) > base64LineLength {
		out.WriteString(encoded[:base64LineLength])
		out.WriteString("\r\n")
		encoded = encoded[base64LineLength:]
	}
	out.WriteString(encoded)
	out.WriteString("\r\n")
	return out.String()
}

func boundaryOpen(builder *strings.Builder, boundary string) {
	fmt.Fprintf(builder, "--%s\r\n", boundary)
}

func boundaryClose(builder *strings.Builder, boundary string) {
	fmt.Fprintf(builder, "--%s--\r\n", boundary)
}
