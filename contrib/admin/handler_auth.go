package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) loginForm(w http.ResponseWriter, r *http.Request) {
	if u := a.currentUser(r); u.CanReachAdmin() {
		view.Redirect(w, r, a.prefix+"/")
		return
	}
	a.render(w, r, http.StatusOK, "login.html", view.Data{
		"Next": view.SafeNext(r.URL.Query().Get("next"), a.prefix+"/"),
	})
}

func (a *Admin) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.PostForm.Get("username"))
	password := r.PostForm.Get("password")
	next := view.SafeNext(r.PostForm.Get("next"), a.prefix+"/")

	user, err := a.app.Auth.AuthenticateRequest(r, username, password)
	if err != nil {
		message, status := "Invalid username or password.", http.StatusUnauthorized
		if errors.Is(err, auth.ErrInactiveAccount) {
			message = "This account has been disabled."
		}
		if errors.Is(err, auth.ErrTooManyAttempts) {
			message, status = "Too many failed attempts. Try again later.", http.StatusTooManyRequests
		}
		a.render(w, r, status, "login.html", view.Data{
			"Error":    message,
			"Username": username,
			"Next":     next,
		})
		return
	}
	if !user.CanReachAdmin() {
		a.render(w, r, http.StatusForbidden, "login.html", view.Data{
			"Error":    "This account does not have access to the admin portal.",
			"Username": username,
			"Next":     next,
		})
		return
	}
	if err := a.app.Auth.Login(r, user); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Flash(r, "success", "Welcome back, "+user.DisplayName()+".")
	view.Redirect(w, r, next)
}

func (a *Admin) logout(w http.ResponseWriter, r *http.Request) {
	_ = a.app.Auth.Logout(r)
	view.Redirect(w, r, a.prefix+"/login")
}
