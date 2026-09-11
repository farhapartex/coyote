package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/view"
)

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
		if err := a.app.Auth.Users().Delete(r.Context(), id); err != nil {
			view.Flash(r, "error", i18n.Tf(r.Context(), "Deleted %d, then stopped: %s", deleted, humanize(r.Context(), err)))
			view.Redirect(w, r, back)
			return
		}
		a.revokeUserSessions(r.Context(), id)
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
		if err := store.DeleteRole(r.Context(), id); err != nil {
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
		if err := sessions.Delete(r.Context(), id); err == nil {
			revoked++
		}
	}
	view.Success(r, i18n.N(r.Context(), "Revoked %d session.", "Revoked %d sessions.", revoked))
	view.Redirect(w, r, back)
}

func (a *Admin) jobBulk(w http.ResponseWriter, r *http.Request) {
	if !a.jobsEnabled() {
		a.notFound(w, r)
		return
	}
	back := a.prefix + "/jobs"
	ids, ok := a.selected(w, r, back)
	if !ok {
		return
	}

	switch r.PostForm.Get("action") {
	case RetryAction:
		a.bulkRetry(w, r, ids, back)
	case DeleteAction:
		a.bulkForget(w, r, ids, back)
	default:
		view.Flash(r, "error", i18n.T(r.Context(), "That action is not available."))
		view.Redirect(w, r, back)
	}
}

func (a *Admin) bulkRetry(w http.ResponseWriter, r *http.Request, ids []string, back string) {
	queued := 0
	for _, id := range ids {
		if err := a.requeue(r, id); err != nil {
			view.Error(r, i18n.Tf(r.Context(), "Queued %d, then failed on %s: %s", queued, id, humanize(r.Context(), err)))
			view.Redirect(w, r, back)
			return
		}
		queued++
	}
	view.Success(r, i18n.N(r.Context(), "Queued %d job to run again.", "Queued %d jobs to run again.", queued))
	view.Redirect(w, r, back)
}

func (a *Admin) bulkForget(w http.ResponseWriter, r *http.Request, ids []string, back string) {
	records, ok := a.storeFor(w, r)
	if !ok {
		return
	}
	schema, err := a.app.Describe(jobs.Record{})
	if err != nil {
		a.fail(w, r, err)
		return
	}

	deleted := 0
	for _, id := range ids {
		if err := records.Delete(r.Context(), schema, id); err != nil {
			view.Error(r, i18n.Tf(r.Context(), "Deleted %d, then failed on %s: %s", deleted, id, humanize(r.Context(), err)))
			view.Redirect(w, r, back)
			return
		}
		deleted++
	}
	view.Success(r, i18n.N(r.Context(), "Deleted %d job.", "Deleted %d jobs.", deleted))
	view.Redirect(w, r, back)
}
