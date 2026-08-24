package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) selected(w http.ResponseWriter, r *http.Request, back string) ([]string, bool) {
	if err := form.Parse(r, uploadMemory); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return nil, false
	}
	ids := r.PostForm["ids"]
	if len(ids) == 0 {
		view.Flash(r, "error", i18n.T(r.Context(), "Nothing was selected."))
		view.Redirect(w, r, back)
		return nil, false
	}
	return ids, true
}

func (a *Admin) userBulk(w http.ResponseWriter, r *http.Request) {
	back := a.prefix + "/users"
	ids, ok := a.selected(w, r, back)
	if !ok {
		return
	}
	if r.PostForm.Get("action") != DeleteAction {
		view.Flash(r, "error", i18n.T(r.Context(), "That action is not available."))
		view.Redirect(w, r, back)
		return
	}

	me := a.currentUser(r)
	deleted, skipped := 0, 0
	for _, id := range ids {
		if me != nil && me.ID == id {
			skipped++
			continue
		}
		if err := a.app.Auth.Users().Delete(id); err != nil {
			view.Flash(r, "error", i18n.Tf(r.Context(), "Deleted %d, then stopped: %s", deleted, humanize(r.Context(), err)))
			view.Redirect(w, r, back)
			return
		}
		a.revokeUserSessions(id)
		deleted++
	}

	message := i18n.N(r.Context(), "Deleted %d user.", "Deleted %d users.", deleted)
	if skipped > 0 {
		message += " " + i18n.T(r.Context(), "Your own account was left alone.")
	}
	view.Success(r, message)
	view.Redirect(w, r, back)
}

func (a *Admin) roleBulk(w http.ResponseWriter, r *http.Request) {
	back := a.prefix + "/roles"
	store, ok := a.permissionStore(w, r)
	if !ok {
		return
	}
	ids, ok := a.selected(w, r, back)
	if !ok {
		return
	}
	if r.PostForm.Get("action") != DeleteAction {
		view.Flash(r, "error", i18n.T(r.Context(), "That action is not available."))
		view.Redirect(w, r, back)
		return
	}

	deleted := 0
	for _, id := range ids {
		if err := store.DeleteRole(id); err != nil {
			view.Flash(r, "error", i18n.Tf(r.Context(), "Deleted %d, then stopped: %s", deleted, humanize(r.Context(), err)))
			view.Redirect(w, r, back)
			return
		}
		deleted++
	}
	view.Success(r, i18n.N(r.Context(), "Deleted %d role.", "Deleted %d roles.", deleted))
	view.Redirect(w, r, back)
}

func (a *Admin) sessionBulk(w http.ResponseWriter, r *http.Request) {
	back := a.prefix + "/sessions"
	sessions, ok := a.sessionStore()
	if !ok {
		a.notFound(w, r)
		return
	}
	ids, ok := a.selected(w, r, back)
	if !ok {
		return
	}

	revoked := 0
	for _, id := range ids {
		if err := sessions.Delete(id); err == nil {
			revoked++
		}
	}
	view.Success(r, i18n.N(r.Context(), "Revoked %d session.", "Revoked %d sessions.", revoked))
	view.Redirect(w, r, back)
}
