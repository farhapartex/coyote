package auth

import (
	"time"

	"github.com/farhapartex/coyote/lib/id"
)

func newUserID() string {
	value, err := id.New()
	if err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return value
}
