package mail

import (
	"fmt"
	"mime"
	netmail "net/mail"
	"sort"
	"strings"
)

const (
	textType = "text/plain; charset=utf-8"
	htmlType = "text/html; charset=utf-8"
)

type headerWriter struct {
	builder strings.Builder
}

func (w *headerWriter) set(name, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(&w.builder, "%s: %s\r\n", name, value)
}

func (w *headerWriter) blank() {
	w.builder.WriteString("\r\n")
}

func (w *headerWriter) String() string {
	return w.builder.String()
}

func encodeWord(value string) string {
	if isASCII(value) {
		return value
	}
	return mime.QEncoding.Encode("utf-8", value)
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] > 127 {
			return false
		}
	}
	return true
}

func addressListHeader(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	parsed, err := ParseAddressList(values)
	if err != nil {
		return "", err
	}
	return FormatAddressList(parsed), nil
}

func addressHeader(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	parsed, err := ParseAddress(value)
	if err != nil {
		return "", err
	}
	return FormatAddressList([]netmail.Address{parsed}), nil
}

func sortedHeaderNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
