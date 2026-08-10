package admin

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/farhapartex/coyote"
	"github.com/farhapartex/coyote/auth"
	"github.com/farhapartex/coyote/session"
)

func (a *Admin) loginForm(w http.ResponseWriter, r *http.Request) {
	if u := a.currentUser(r); u != nil && u.IsStaff {
		coyote.Redirect(w, r, a.prefix+"/")
		return
	}
	a.render(w, r, http.StatusOK, "login.html", coyote.Data{
		"Next": safeNext(r.URL.Query().Get("next"), a.prefix+"/"),
	})
}

func (a *Admin) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.PostForm.Get("username"))
	password := r.PostForm.Get("password")
	next := safeNext(r.PostForm.Get("next"), a.prefix+"/")

	user, err := a.app.Auth.Authenticate(username, password)
	if err != nil {
		message := "Invalid username or password."
		if errors.Is(err, auth.ErrInactiveAccount) {
			message = "This account has been disabled."
		}
		a.render(w, r, http.StatusUnauthorized, "login.html", coyote.Data{
			"Error":    message,
			"Username": username,
			"Next":     next,
		})
		return
	}
	if !user.IsStaff {
		a.render(w, r, http.StatusForbidden, "login.html", coyote.Data{
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
	coyote.Flash(r, "success", "Welcome back, "+user.DisplayName()+".")
	coyote.Redirect(w, r, next)
}

func (a *Admin) logout(w http.ResponseWriter, r *http.Request) {
	_ = a.app.Auth.Logout(r)
	coyote.Redirect(w, r, a.prefix+"/login")
}

func (a *Admin) dashboard(w http.ResponseWriter, r *http.Request) {
	users := a.app.Auth.Users().All()
	staff := 0
	for _, u := range users {
		if u.IsStaff {
			staff++
		}
	}
	a.render(w, r, http.StatusOK, "dashboard.html", coyote.Data{
		"Nav":          "dashboard",
		"UserCount":    len(users),
		"StaffCount":   staff,
		"SessionCount": a.sessionStore().Count(),
		"RouteCount":   len(a.app.Routes()),
		"Uptime":       time.Since(a.app.Started).Round(time.Second).String(),
		"Recent":       recentUsers(users, 5),
	})
}

func (a *Admin) userList(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	all := a.app.Auth.Users().All()
	matched := make([]*auth.User, 0, len(all))
	for _, u := range all {
		if query == "" ||
			strings.Contains(strings.ToLower(u.Username), query) ||
			strings.Contains(strings.ToLower(u.Email), query) ||
			strings.Contains(strings.ToLower(u.FullName), query) {
			matched = append(matched, u)
		}
	}
	a.render(w, r, http.StatusOK, "users.html", coyote.Data{
		"Nav":   "users",
		"Users": matched,
		"Query": r.URL.Query().Get("q"),
		"Total": len(all),
	})
}

func (a *Admin) userForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data := coyote.Data{"Nav": "users", "IsNew": true, "Form": &auth.User{IsActive: true}}
	if id != "" {
		user, err := a.app.Auth.Users().ByID(id)
		if err != nil {
			a.notFound(w, r)
			return
		}
		data["IsNew"] = false
		data["Form"] = user
	}
	a.render(w, r, http.StatusOK, "user_form.html", data)
}

func (a *Admin) userCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	form := &auth.User{
		Username:    strings.TrimSpace(r.PostForm.Get("username")),
		Email:       strings.TrimSpace(r.PostForm.Get("email")),
		FullName:    strings.TrimSpace(r.PostForm.Get("full_name")),
		IsActive:    r.PostForm.Get("is_active") != "",
		IsStaff:     r.PostForm.Get("is_staff") != "",
		IsSuperuser: r.PostForm.Get("is_superuser") != "",
	}
	password := r.PostForm.Get("password")

	user, err := a.app.Auth.CreateUser(form.Username, form.Email, password, form.IsStaff, form.IsSuperuser)
	if err != nil {
		a.render(w, r, http.StatusBadRequest, "user_form.html", coyote.Data{
			"Nav": "users", "IsNew": true, "Form": form, "Error": humanize(err),
		})
		return
	}
	user.FullName = form.FullName
	user.IsActive = form.IsActive
	if err := a.app.Auth.Users().Update(user); err != nil {
		a.render(w, r, http.StatusBadRequest, "user_form.html", coyote.Data{
			"Nav": "users", "IsNew": true, "Form": form, "Error": humanize(err),
		})
		return
	}
	coyote.Flash(r, "success", "User "+user.Username+" created.")
	coyote.Redirect(w, r, a.prefix+"/users")
}

func (a *Admin) userUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	user, err := a.app.Auth.Users().ByID(id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	me := a.currentUser(r)
	isSelf := me != nil && me.ID == user.ID

	user.Username = strings.TrimSpace(r.PostForm.Get("username"))
	user.Email = strings.TrimSpace(r.PostForm.Get("email"))
	user.FullName = strings.TrimSpace(r.PostForm.Get("full_name"))
	if !isSelf {
		user.IsActive = r.PostForm.Get("is_active") != ""
		user.IsStaff = r.PostForm.Get("is_staff") != ""
		user.IsSuperuser = r.PostForm.Get("is_superuser") != ""
	}

	fail := func(err error) {
		a.render(w, r, http.StatusBadRequest, "user_form.html", coyote.Data{
			"Nav": "users", "IsNew": false, "Form": user, "Error": humanize(err),
		})
	}

	if password := r.PostForm.Get("password"); password != "" {
		if err := auth.ValidatePassword(password); err != nil {
			fail(err)
			return
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			fail(err)
			return
		}
		user.PasswordHash = hash
		a.sessionStore().DeleteByUserID(user.ID)
	}

	if err := a.app.Auth.Users().Update(user); err != nil {
		fail(err)
		return
	}
	coyote.Flash(r, "success", "User "+user.Username+" updated.")
	coyote.Redirect(w, r, a.prefix+"/users")
}

func (a *Admin) userDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	me := a.currentUser(r)
	if me != nil && me.ID == id {
		coyote.Flash(r, "error", "You cannot delete your own account.")
		coyote.Redirect(w, r, a.prefix+"/users")
		return
	}
	if err := a.app.Auth.Users().Delete(id); err != nil {
		coyote.Flash(r, "error", humanize(err))
		coyote.Redirect(w, r, a.prefix+"/users")
		return
	}
	a.sessionStore().DeleteByUserID(id)
	coyote.Flash(r, "success", "User deleted.")
	coyote.Redirect(w, r, a.prefix+"/users")
}

type sessionRow struct {
	ID       string
	Short    string
	Username string
	Created  time.Time
	Expires  time.Time
	IsSelf   bool
}

func (a *Admin) sessionList(w http.ResponseWriter, r *http.Request) {
	current := session.FromRequest(r)
	rows := []sessionRow{}
	for _, s := range a.sessionStore().All() {
		username := "anonymous"
		if id := s.UserID(); id != "" {
			if u, err := a.app.Auth.Users().ByID(id); err == nil {
				username = u.Username
			} else {
				username = "unknown"
			}
		}
		rows = append(rows, sessionRow{
			ID:       s.ID(),
			Short:    truncate(s.ID(), 16),
			Username: username,
			Created:  s.CreatedAt(),
			Expires:  s.ExpiresAt(),
			IsSelf:   current != nil && current.ID() == s.ID(),
		})
	}
	a.render(w, r, http.StatusOK, "sessions.html", coyote.Data{
		"Nav":      "sessions",
		"Sessions": rows,
	})
}

func (a *Admin) sessionRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	current := session.FromRequest(r)
	if current != nil && current.ID() == id {
		coyote.Flash(r, "error", "Use log out to end your own session.")
		coyote.Redirect(w, r, a.prefix+"/sessions")
		return
	}
	_ = a.sessionStore().Delete(id)
	coyote.Flash(r, "success", "Session revoked.")
	coyote.Redirect(w, r, a.prefix+"/sessions")
}

func (a *Admin) routeList(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "routes.html", coyote.Data{
		"Nav":    "routes",
		"Routes": a.app.Routes(),
	})
}

func (a *Admin) notFound(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusNotFound, "notfound.html", coyote.Data{"Nav": ""})
}

func recentUsers(users []*auth.User, n int) []*auth.User {
	sorted := append([]*auth.User{}, users...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].CreatedAt.After(sorted[j-1].CreatedAt); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

func humanize(err error) string {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		return "That username is already taken."
	case errors.Is(err, auth.ErrInvalidUser):
		return "Usernames must be 3-64 characters using letters, digits or . _ @ + -"
	case errors.Is(err, auth.ErrPasswordTooShort):
		return "Passwords must be at least 8 characters."
	case errors.Is(err, auth.ErrLastSuperuser):
		return "You cannot remove the last active superuser."
	case errors.Is(err, auth.ErrUserNotFound):
		return "That user no longer exists."
	default:
		return err.Error()
	}
}

func safeNext(next, fallback string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return fallback
	}
	return next
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
