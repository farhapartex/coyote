package admin

import (
	"errors"
	"net/http"

	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) storeFor(w http.ResponseWriter, r *http.Request) (model.Store, bool) {
	records, err := a.app.Store()
	if err != nil {
		a.fail(w, r, err)
		return nil, false
	}
	return records, true
}

func (a *Admin) writableStore(w http.ResponseWriter, r *http.Request, entry managed) (model.Store, bool) {
	if entry.readOnly {
		view.Error(r, i18n.Tf(r.Context(), "%s are read only.", entry.schema.Plural))
		view.Redirect(w, r, a.prefix+"/"+entry.Slug())
		return nil, false
	}
	return a.storeFor(w, r)
}

func (a *Admin) notFoundOrFail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		a.notFound(w, r)
		return
	}
	a.fail(w, r, err)
}

func (a *Admin) fail(w http.ResponseWriter, r *http.Request, err error) {
	a.app.Logger.Error("admin request failed", "path", r.URL.Path, "error", err)
	if a.app.Settings.Debug {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
}
