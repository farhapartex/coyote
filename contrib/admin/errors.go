package admin

import (
	"context"
	"errors"
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) notFound(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusNotFound, "notfound.html", view.Data{"Nav": ""})
}

func humanize(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		return i18n.T(ctx, "That username is already taken.")
	case errors.Is(err, auth.ErrInvalidUser):
		return i18n.T(ctx, "Usernames must be 3-64 characters using letters, digits or . _ @ + -")
	case errors.Is(err, auth.ErrPasswordTooShort):
		return i18n.T(ctx, "Passwords must be at least 8 characters.")
	case errors.Is(err, auth.ErrLastSuperadmin):
		return i18n.T(ctx, "You cannot remove or disable the last active superadmin.")
	case errors.Is(err, auth.ErrEmailExists):
		return i18n.T(ctx, "That email address is already registered.")
	case errors.Is(err, auth.ErrInvalidEmail):
		return i18n.T(ctx, "That does not look like a valid email address.")
	case errors.Is(err, auth.ErrUserNotFound):
		return i18n.T(ctx, "That user no longer exists.")
	default:
		return err.Error()
	}
}
