package text

import "unicode/utf8"

const ellipsis = "…"

func Truncate(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + ellipsis
}
