package admin

import (
	"errors"
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) notFound(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusNotFound, "notfound.html", view.Data{"Nav": ""})
}

func humanize(err error) string {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		return "That username is already taken."
	case errors.Is(err, auth.ErrInvalidUser):
		return "Usernames must be 3-64 characters using letters, digits or . _ @ + -"
	case errors.Is(err, auth.ErrPasswordTooShort):
		return "Passwords must be at least 8 characters."
	case errors.Is(err, auth.ErrLastSuperadmin):
		return "You cannot remove or disable the last active superadmin."
	case errors.Is(err, auth.ErrEmailExists):
		return "That email address is already registered."
	case errors.Is(err, auth.ErrInvalidEmail):
		return "That does not look like a valid email address."
	case errors.Is(err, auth.ErrUserNotFound):
		return "That user no longer exists."
	default:
		return err.Error()
	}
}
