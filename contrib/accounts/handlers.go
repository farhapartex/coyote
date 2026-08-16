package accounts

import (
	"errors"
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Accounts) loginForm(w http.ResponseWriter, r *http.Request) {
	if a.app.Auth.CurrentUser(r) != nil {
		view.Redirect(w, r, a.afterIn)
		return
	}
	a.render(w, r, http.StatusOK, "login.html", a.pages.Login, view.Data{
		"Title": "Sign in",
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
		message, status := "Invalid username or password.", http.StatusUnauthorized
		if errors.Is(err, auth.ErrInactiveAccount) {
			message = "This account has been disabled."
		}
		if errors.Is(err, auth.ErrTooManyAttempts) {
			message, status = "Too many failed attempts. Try again later.", http.StatusTooManyRequests
		}
		a.render(w, r, status, "login.html", a.pages.Login, view.Data{
			"Title": "Sign in", "Error": message, "Username": username, "Next": next,
		})
		return
	}

	if err := a.app.Auth.Login(r, user); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Success(r, "Welcome back, "+user.DisplayName()+".")
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
		"Title": "Create an account",
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
			"Title": "Create an account", "Form": form, "Error": humanize(err),
		})
		return
	}

	if err := a.app.Auth.Login(r, user); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Success(r, "Welcome, "+user.DisplayName()+".")
	view.Redirect(w, r, a.afterIn)
}

func (a *Accounts) profileForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "profile.html", a.pages.Profile, view.Data{
		"Title": "Your profile",
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
			"Title": "Your profile", "Form": updated, "Error": humanize(err),
		})
		return
	}
	view.Success(r, "Profile saved.")
	view.Redirect(w, r, a.prefix+"/profile")
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
			"Title": "Your profile", "Form": user, "PasswordError": message,
		})
	}

	if _, err := a.app.Auth.Authenticate(user.Username, r.PostForm.Get("current_password")); err != nil {
		fail("Your current password is not right.")
		return
	}
	if err := a.app.Auth.SetPassword(user.ID, r.PostForm.Get("new_password")); err != nil {
		fail(humanize(err))
		return
	}

	if err := a.app.Auth.Login(r, user); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	view.Success(r, "Password changed.")
	view.Redirect(w, r, a.prefix+"/profile")
}

func humanize(err error) string {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		return "That username is already taken."
	case errors.Is(err, auth.ErrEmailExists):
		return "That email is already registered."
	case errors.Is(err, auth.ErrInvalidUser):
		return "Usernames may use letters, digits, dots, hyphens and underscores."
	case errors.Is(err, auth.ErrInvalidEmail):
		return "That does not look like an email address."
	case errors.Is(err, auth.ErrPasswordRejected):
		return strings.ReplaceAll(strings.ReplaceAll(err.Error(), "coyote/auth: ", ""), "\n", "; ")
	}
	return "Something went wrong. Please try again."
}
