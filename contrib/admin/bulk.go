package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

const (
	DeleteAction = "delete"
	maxBulkIDs   = 1000
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
	if len(ids) > maxBulkIDs {
		view.Flash(r, "error", i18n.Tf(r.Context(),
			"That is %d rows; %d is the most one action can take at a time.", len(ids), maxBulkIDs))
		view.Redirect(w, r, back)
		return nil, false
	}
	return ids, true
}

func (a *Admin) resourceBulk(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if entry.readOnly {
			a.render(w, r, http.StatusForbidden, "forbidden.html", nil)
			return
		}

		back := a.prefix + "/" + entry.Slug()
		ids, ok := a.selected(w, r, back)
		if !ok {
			return
		}

		name := r.PostForm.Get("action")
		if name == DeleteAction {
			a.bulkDelete(w, r, entry, ids, back)
			return
		}

		for _, action := range entry.actions {
			if action.Name != name || action.Run == nil {
				continue
			}
			if !a.may(r, entry, action.permission()) {
				a.render(w, r, http.StatusForbidden, "forbidden.html", nil)
				return
			}
			affected, err := action.Run(r, ids)
			if err != nil {
				view.Flash(r, "error", humanize(r.Context(), err))
				view.Redirect(w, r, back)
				return
			}
			view.Success(r, i18n.Tf(r.Context(), "%s applied to %d %s.", action.Label, affected, entry.Title()))
			view.Redirect(w, r, back)
			return
		}

		view.Flash(r, "error", i18n.T(r.Context(), "That action is not available."))
		view.Redirect(w, r, back)
	}
}

func (a *Admin) bulkDelete(w http.ResponseWriter, r *http.Request, entry managed, ids []string, back string) {
	if !a.may(r, entry, auth.ActionDelete) {
		a.render(w, r, http.StatusForbidden, "forbidden.html", nil)
		return
	}
	records, ok := a.storeFor(w, r)
	if !ok {
		return
	}

	deleted := 0
	for _, id := range ids {
		if err := records.Delete(r.Context(), entry.schema, id); err != nil {
			view.Flash(r, "error", i18n.Tf(r.Context(), "Deleted %d, then failed on %s: %s", deleted, id, humanize(r.Context(), err)))
			view.Redirect(w, r, back)
			return
		}
		deleted++
	}

	view.Success(r, i18n.Tf(r.Context(), "Deleted %d %s.", deleted, entry.Title()))
	view.Redirect(w, r, back)
}
