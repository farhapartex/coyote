package admin

import (
	"fmt"
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/view"
)

const DeleteAction = "delete"

func (a *Admin) resourceBulk(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if entry.readOnly {
			a.render(w, r, http.StatusForbidden, "forbidden.html", nil)
			return
		}
		if err := form.Parse(r, uploadMemory); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		ids := r.PostForm["ids"]
		back := a.prefix + "/" + entry.Slug()
		if len(ids) == 0 {
			view.Flash(r, "error", "Nothing was selected.")
			view.Redirect(w, r, back)
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
			affected, err := action.Run(r, ids)
			if err != nil {
				view.Flash(r, "error", humanize(err))
				view.Redirect(w, r, back)
				return
			}
			view.Success(r, fmt.Sprintf("%s applied to %d %s.", action.Label, affected, entry.Title()))
			view.Redirect(w, r, back)
			return
		}

		view.Flash(r, "error", "That action is not available.")
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
			view.Flash(r, "error", fmt.Sprintf("Deleted %d, then failed on %s: %s", deleted, id, humanize(err)))
			view.Redirect(w, r, back)
			return
		}
		deleted++
	}

	view.Success(r, fmt.Sprintf("Deleted %d %s.", deleted, entry.Title()))
	view.Redirect(w, r, back)
}
