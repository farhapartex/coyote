package text

import (
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(name string) string {
	slug := nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "_")
	return strings.Trim(slug, "_")
}

func Hyphenate(name string) string {
	slug := nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	return strings.Trim(slug, "-")
}
