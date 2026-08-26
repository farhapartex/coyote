package mail

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const defaultAttachmentType = "application/octet-stream"

type Attachment struct {
	Filename string
	Type     string
	Content  []byte
}

func NewAttachment(filename, contentType string, content []byte) Attachment {
	return Attachment{Filename: filename, Type: contentType, Content: content}
}

func FileAttachment(path string) (Attachment, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, fmt.Errorf("coyote/mail: reading %s: %w", path, err)
	}
	return Attachment{Filename: filepath.Base(path), Content: content}, nil
}

func (a Attachment) Name() string {
	name := filepath.Base(strings.TrimSpace(a.Filename))
	switch name {
	case "", ".", "..", string(filepath.Separator):
		return "attachment"
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '"' || r == '\\' {
			return -1
		}
		return r
	}, name)
}

func (a Attachment) ContentType() string {
	if declared := strings.TrimSpace(a.Type); declared != "" {
		return declared
	}
	if byExtension := mime.TypeByExtension(filepath.Ext(a.Name())); byExtension != "" {
		return byExtension
	}
	if len(a.Content) > 0 {
		return http.DetectContentType(a.Content)
	}
	return defaultAttachmentType
}

func (a Attachment) Validate() error {
	if len(a.Content) == 0 {
		return fmt.Errorf("coyote/mail: attachment %q is empty", a.Name())
	}
	if containsBreak(a.Type) {
		return fmt.Errorf("%w: attachment content type", ErrHeaderInjection)
	}
	if containsBreak(a.Filename) {
		return fmt.Errorf("%w: attachment filename", ErrHeaderInjection)
	}
	return nil
}
