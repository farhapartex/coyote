package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) render(w http.ResponseWriter, r *http.Request, status int, page string, data view.Data) {
	if data == nil {
		data = view.Data{}
	}
	a.app.Context(r, data)
	data["Prefix"] = a.prefix
	data["SiteName"] = a.siteName
	data["Tagline"] = a.tagline
	data["Sections"] = a.sections
	data["Resources"] = a.visibleResources(r)
	data["Permissions"] = a.app.Auth.Permissions() != nil
	if err := a.templates.Render(w, status, "templates/"+page, data); err != nil {
		a.app.Logger.Error("admin render failed: " + err.Error())
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}
