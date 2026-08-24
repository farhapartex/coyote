package i18n

import "strings"

func SplitPrefix(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/")
	segment, rest, found := strings.Cut(trimmed, "/")
	if !found {
		return segment, "/"
	}
	return segment, "/" + rest
}

func WithPrefix(path, tag string) string {
	if tag == "" {
		return path
	}
	if path == "" || path == "/" {
		return "/" + tag + "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "/" + tag + path
}

func (l *Locale) Path(path string) string {
	if l == nil || l.bundle == nil || l.tag == l.bundle.Default() {
		return path
	}
	return WithPrefix(path, l.tag)
}

func (l *Locale) Switch(path, tag string) string {
	if l == nil || l.bundle == nil {
		return path
	}
	target := Normalise(tag)
	if !l.bundle.Supports(target) {
		return path
	}

	segment, rest := SplitPrefix(path)
	if l.bundle.Supports(segment) {
		path = rest
	}
	if target == l.bundle.Default() {
		return path
	}
	return WithPrefix(path, target)
}
