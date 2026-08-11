package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) routeList(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "routes.html", view.Data{
		"Nav":    "routes",
		"Routes": a.app.Routes(),
	})
}
