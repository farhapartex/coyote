package auth

import (
	"strings"

	"github.com/farhapartex/coyote/lib/text"
)

type Stats struct {
	Total       int
	Superadmins int
	Staff       int
}

func Matches(u *User, term string) bool {
	folded := text.Fold(term)
	if folded == "" {
		return true
	}
	for _, field := range []string{u.Username, u.Email, u.FirstName, u.LastName} {
		if strings.Contains(text.Fold(field), folded) {
			return true
		}
	}
	return false
}

func Window(users []*User, limit, offset int) []*User {
	if offset > len(users) {
		offset = len(users)
	}
	users = users[offset:]
	if limit > 0 && limit < len(users) {
		users = users[:limit]
	}
	return users
}
