package main

import (
	"net/http"

	"github.com/farhapartex/coyote/core/app"
)

func render(a *app.App, w http.ResponseWriter, r *http.Request, page string, data app.Data) {
	a.Render(w, r, page, decorate(a, r, data))
}

func renderStatus(a *app.App, w http.ResponseWriter, r *http.Request, status int, page string, data app.Data) {
	a.RenderStatus(w, r, status, page, decorate(a, r, data))
}

func decorate(a *app.App, r *http.Request, data app.Data) app.Data {
	if data == nil {
		data = app.Data{}
	}
	data.SetDefault("Title", "")
	data.SetDefault("Nav", "")
	data.SetDefault("Categories", categoryLinks(r.Context(), a))
	data.SetDefault("CartCount", cartCount(r))
	return data
}
