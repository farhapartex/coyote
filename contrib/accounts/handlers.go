package accounts

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Accounts) loginForm(w http.ResponseWriter, r *http.Request) {
	if a.app.Auth.CurrentUser(r) != nil {
		view.Redirect(w, r, a.afterIn)
		return
	}
	a.render(w, r, http.StatusOK, "login.html", a.pages.Login, view.Data{
		"Title": i18n.T(r.Context(), "Sign in"),
		"Next":  view.SafeNext(r.URL.Query().Get("next"), a.afterIn),
	})
}

func (a *Accounts) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.PostForm.Get("username"))
	next := view.SafeNext(r.PostForm.Get("next"), a.afterIn)

	user, err := a.app.Auth.AuthenticateRequest(r, username, r.PostForm.Get("password"))
	if err != nil {
		message, status := i18n.T(r.Context(), "Invalid username or password."), http.StatusUnauthorized
		if errors.Is(err, auth.ErrInactiveAccount) {
			message = i18n.T(r.Context(), "This account has been disabled.")
		}
		if errors.Is(err, auth.ErrTooManyAttempts) {
			message, status = i18n.T(r.Context(), "Too many failed attempts. Try again later."), http.StatusTooManyRequests
		}
		a.render(w, r, status, "login.html", a.pages.Login, view.Data{
			"Title": i18n.T(r.Context(), "Sign in"), "Error": message, "Username": username, "Next": next,
		})
		return
	}

	if err := a.app.Auth.Login(r, user); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Success(r, i18n.Tf(r.Context(), "Welcome back, %s.", user.DisplayName()))
	view.Redirect(w, r, next)
}

func (a *Accounts) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.app.Auth.Logout(r); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Redirect(w, r, a.afterOut)
}

func (a *Accounts) registerForm(w http.ResponseWriter, r *http.Request) {
	if a.app.Auth.CurrentUser(r) != nil {
		view.Redirect(w, r, a.afterIn)
		return
	}
	a.render(w, r, http.StatusOK, "register.html", a.pages.Register, view.Data{
		"Title": i18n.T(r.Context(), "Create an account"),
		"Form":  &auth.User{},
	})
}

func (a *Accounts) registerSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}

	form := &auth.User{
		Username:  strings.TrimSpace(r.PostForm.Get("username")),
		Email:     strings.TrimSpace(r.PostForm.Get("email")),
		FirstName: strings.TrimSpace(r.PostForm.Get("first_name")),
		LastName:  strings.TrimSpace(r.PostForm.Get("last_name")),
	}

	user, err := a.app.Auth.CreateUser(auth.NewUser{
		Username:  form.Username,
		Email:     form.Email,
		FirstName: form.FirstName,
		LastName:  form.LastName,
		Password:  r.PostForm.Get("password"),
	})
	if err != nil {
		a.render(w, r, http.StatusBadRequest, "register.html", a.pages.Register, view.Data{
			"Title": i18n.T(r.Context(), "Create an account"), "Form": form, "Error": humanize(r.Context(), err),
		})
		return
	}

	if err := a.app.Auth.Login(r, user); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Success(r, i18n.Tf(r.Context(), "Welcome, %s.", user.DisplayName()))
	view.Redirect(w, r, a.afterIn)
}

func (a *Accounts) profileForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "profile.html", a.pages.Profile, view.Data{
		"Title": i18n.T(r.Context(), "Your profile"),
		"Wide":  true,
		"Form":  a.app.Auth.CurrentUser(r),
	})
}

func (a *Accounts) profileSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	user := a.app.Auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "401 unauthorized", http.StatusUnauthorized)
		return
	}

	updated := user.Clone()
	updated.FirstName = strings.TrimSpace(r.PostForm.Get("first_name"))
	updated.LastName = strings.TrimSpace(r.PostForm.Get("last_name"))
	updated.Email = strings.TrimSpace(r.PostForm.Get("email"))

	if err := a.app.Auth.Users().Update(updated); err != nil {
		a.render(w, r, http.StatusBadRequest, "profile.html", a.pages.Profile, view.Data{
			"Title": i18n.T(r.Context(), "Your profile"), "Form": updated, "Error": humanize(r.Context(), err),
		})
		return
	}
	view.Success(r, i18n.T(r.Context(), "Profile saved."))
	view.Redirect(w, r, a.prefix+"/profile")
}

func (a *Accounts) passwordForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "profile.html", a.pages.Profile, view.Data{
		"Title": i18n.T(r.Context(), "Your profile"),
		"Wide":  true,
		"Form":  a.app.Auth.CurrentUser(r),
	})
}

func (a *Accounts) passwordSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	user := a.app.Auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "401 unauthorized", http.StatusUnauthorized)
		return
	}

	fail := func(message string) {
		a.render(w, r, http.StatusBadRequest, "profile.html", a.pages.Profile, view.Data{
			"Title": i18n.T(r.Context(), "Your profile"), "Form": user, "PasswordError": message,
		})
	}

	err := a.app.Auth.ChangePassword(r, auth.PasswordChange{
		Current: r.PostForm.Get("current_password"),
		New:     r.PostForm.Get("new_password"),
		Confirm: r.PostForm.Get("confirm_password"),
	})
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		fail(i18n.T(r.Context(), "Your current password is not right."))
		return
	case errors.Is(err, auth.ErrPasswordMismatch):
		fail(i18n.T(r.Context(), "The new passwords do not match."))
		return
	case errors.Is(err, auth.ErrTooManyAttempts):
		fail(i18n.T(r.Context(), "Too many attempts. Try again later."))
		return
	case err != nil:
		fail(humanize(r.Context(), err))
		return
	}

	a.app.Auth.RevokeOtherSessions(r)
	view.Success(r, i18n.T(r.Context(), "Password changed."))
	view.Redirect(w, r, a.prefix+"/profile")
}

func humanize(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		return i18n.T(ctx, "That username is already taken.")
	case errors.Is(err, auth.ErrEmailExists):
		return i18n.T(ctx, "That email is already registered.")
	case errors.Is(err, auth.ErrInvalidUser):
		return i18n.T(ctx, "Usernames may use letters, digits, dots, hyphens and underscores.")
	case errors.Is(err, auth.ErrInvalidEmail):
		return i18n.T(ctx, "That does not look like an email address.")
	case errors.Is(err, auth.ErrPasswordRejected):
		return strings.ReplaceAll(strings.ReplaceAll(err.Error(), "coyote/auth: ", ""), "\n", "; ")
	}
	return i18n.T(ctx, "Something went wrong. Please try again.")
}
