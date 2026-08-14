package text

import "strings"

func Fold(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func Blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
