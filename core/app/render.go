package app

import (
	"log/slog"
	"net/http"

	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/session"
)

func (a *App) Render(w http.ResponseWriter, r *http.Request, page string, data Data) {
	a.RenderStatus(w, r, http.StatusOK, page, data)
}

func (a *App) RenderStatus(w http.ResponseWriter, r *http.Request, status int, page string, data Data) {
	if data == nil {
		data = Data{}
	}
	a.Context(r, data)
	if err := a.Templates.Render(w, status, page, data); err != nil {
		a.Logger.Error("render failed", slog.String("page", page), slog.Any("error", err))
		if a.Settings.Debug {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

func (a *App) Context(r *http.Request, data Data) Data {
	if data == nil {
		data = Data{}
	}
	sess := session.FromRequest(r)
	data.SetDefault("Request", r)
	data.SetDefault("Path", r.URL.Path)
	data.SetDefault("User", a.Auth.CurrentUser(r))
	data.SetDefault("Session", sess)
	data.SetDefault("CSRFToken", a.Sessions.CSRFToken(r))
	data.SetDefault("Version", Version)
	data.SetDefault("Debug", a.Settings.Debug)
	data.SetDefault("Nonce", middleware.NonceFrom(r.Context()))
	if sess != nil {
		data.SetDefault("Flashes", sess.Flashes())
	}
	return data
}
