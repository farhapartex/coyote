package upload

import (
	"errors"
	"fmt"
	"image"
	"mime"
	"net/http"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

var (
	ErrTooLarge     = errors.New("coyote/upload: file is larger than the limit")
	ErrTypeNotAllow = errors.New("coyote/upload: file type is not allowed")
	ErrTypeMismatch = errors.New("coyote/upload: declared type does not match the content")
	ErrEmptyFile    = errors.New("coyote/upload: file is empty")
	ErrTooManyPixel = errors.New("coyote/upload: image dimensions are too large")
)

const sniffLength = 512

type Rules struct {
	MaxSize   int64
	Allowed   []string
	MaxPixels int
}

func (r Rules) allows(kind string) bool {
	if len(r.Allowed) == 0 {
		return false
	}
	kind = normalise(kind)
	for _, candidate := range r.Allowed {
		candidate = normalise(candidate)
		if candidate == kind {
			return true
		}
		if strings.HasSuffix(candidate, "/*") && strings.HasPrefix(kind, strings.TrimSuffix(candidate, "*")) {
			return true
		}
	}
	return false
}

func normalise(kind string) string {
	parsed, _, err := mime.ParseMediaType(kind)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(kind))
	}
	return strings.ToLower(parsed)
}

func Sniff(head []byte) string {
	return normalise(http.DetectContentType(head))
}

func (r Rules) Check(head []byte, declared string) (string, error) {
	if len(head) == 0 {
		return "", ErrEmptyFile
	}

	sniffed := Sniff(head)
	if !r.allows(sniffed) {
		return "", fmt.Errorf("%w: %s", ErrTypeNotAllow, sniffed)
	}
	if declared != "" && normalise(declared) != sniffed && !compatible(normalise(declared), sniffed) {
		return "", fmt.Errorf("%w: sent %s, looks like %s", ErrTypeMismatch, normalise(declared), sniffed)
	}
	if err := r.checkPixels(head, sniffed); err != nil {
		return "", err
	}
	return sniffed, nil
}

func compatible(declared, sniffed string) bool {
	if declared == "" || declared == "application/octet-stream" {
		return true
	}
	if sniffed == "application/octet-stream" || strings.HasPrefix(sniffed, "text/plain") {
		return !strings.HasPrefix(declared, "image/")
	}
	return false
}

func (r Rules) checkPixels(head []byte, sniffed string) error {
	if r.MaxPixels <= 0 || !strings.HasPrefix(sniffed, "image/") {
		return nil
	}
	config, _, err := image.DecodeConfig(strings.NewReader(string(head)))
	if err != nil {
		return nil
	}
	if config.Width*config.Height > r.MaxPixels {
		return fmt.Errorf("%w: %dx%d", ErrTooManyPixel, config.Width, config.Height)
	}
	return nil
}

func ExtensionFor(kind string) string {
	switch normalise(kind) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	case "text/plain; charset=utf-8", "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "application/zip":
		return ".zip"
	}
	if found, err := mime.ExtensionsByType(normalise(kind)); err == nil && len(found) > 0 {
		return found[0]
	}
	return ".bin"
}
