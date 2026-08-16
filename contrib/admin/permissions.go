package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
)

func (a *Admin) may(r *http.Request, entry managed, action string) bool {
	if a.app.Auth.Permissions() == nil {
		return true
	}
	return a.app.Auth.Can(r, auth.Codename(entry.schema.Table, action))
}

func (a *Admin) permit(entry managed, action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.may(r, entry, action) {
			a.render(w, r, http.StatusForbidden, "forbidden.html", nil)
			return
		}
		next(w, r)
	}
}

func (a *Admin) visibleResources(r *http.Request) []managed {
	all := a.resources.all()
	if a.app.Auth.Permissions() == nil {
		return all
	}
	out := make([]managed, 0, len(all))
	for _, entry := range all {
		if a.may(r, entry, auth.ActionRead) {
			out = append(out, entry)
		}
	}
	return out
}
