package main

import (
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

const demoKey = "demo:note"

func cachedPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stored, found, err := a.Cache().Get(r.Context(), demoKey)
		message := ""
		if err != nil {
			message = err.Error()
		}

		pages, _ := a.CacheByAlias("pages")
		fromPages := ""
		if pages != nil {
			if raw, ok, _ := pages.Get(r.Context(), demoKey); ok {
				fromPages = string(raw)
			}
		}

		a.Render(w, r, "pages/cached.html", view.Data{
			"Title":  i18n.T(r.Context(), "Cache"),
			"Value":  string(stored),
			"Found":  found,
			"Pages":  fromPages,
			"Error":  message,
			"Stats":  a.CacheStats(),
			"Report": nil,
		})
	}
}

func cachedSave(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		note := r.PostForm.Get("note")
		switch r.PostForm.Get("action") {
		case "clear":
			if err := a.Cache().Clear(r.Context()); err != nil {
				view.Error(r, err.Error())
			} else {
				view.Success(r, "The default cache was cleared. The other alias was untouched.")
			}
		case "pages":
			pages, err := a.CacheByAlias("pages")
			if err != nil {
				view.Error(r, err.Error())
				break
			}
			if err := pages.Set(r.Context(), demoKey, []byte(note), time.Minute); err != nil {
				view.Error(r, err.Error())
			} else {
				view.Success(r, "Stored under the same key in the pages cache.")
			}
		default:
			if err := a.Cache().Set(r.Context(), demoKey, []byte(note), time.Minute); err != nil {
				view.Error(r, err.Error())
			} else {
				view.Success(r, "Stored for a minute in the default cache.")
			}
		}
		view.Redirect(w, r, "/cached")
	}
}
