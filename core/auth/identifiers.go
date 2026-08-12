package auth

import (
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/id"
)

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func newUserID() string {
	value, err := id.New()
	if err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return value
}
