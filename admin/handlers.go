package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) loginForm(w http.ResponseWriter, r *http.Request) {
	if u := a.currentUser(r); u != nil && u.IsSuperadmin {
		view.Redirect(w, r, a.prefix+"/")
		return
	}
	a.render(w, r, http.StatusOK, "login.html", view.Data{
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
		a.render(w, r, http.StatusUnauthorized, "login.html", view.Data{
			"Error":    message,
			"Username": username,
			"Next":     next,
		})
		return
	}
	if !user.IsSuperadmin {
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

func (a *Admin) dashboard(w http.ResponseWriter, r *http.Request) {
	users := a.app.Auth.Users().All()
	superadmins := 0
	for _, u := range users {
		if u.IsSuperadmin {
			superadmins++
		}
	}
	a.render(w, r, http.StatusOK, "dashboard.html", view.Data{
		"Nav":             "dashboard",
		"UserCount":       len(users),
		"SuperadminCount": superadmins,
		"SessionCount":    a.sessionCount(),
		"RouteCount":      len(a.app.Routes()),
		"Uptime":          time.Since(a.app.Started).Round(time.Second).String(),
		"Recent":          recentUsers(users, 5),
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
			strings.Contains(strings.ToLower(u.FullName()), query) {
			matched = append(matched, u)
		}
	}
	a.render(w, r, http.StatusOK, "users.html", view.Data{
		"Nav":   "users",
		"Users": matched,
		"Query": r.URL.Query().Get("q"),
		"Total": len(all),
	})
}

func (a *Admin) userForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data := view.Data{"Nav": "users", "IsNew": true, "Form": &auth.User{IsActive: true}}
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
		Username:     strings.TrimSpace(r.PostForm.Get("username")),
		Email:        strings.TrimSpace(r.PostForm.Get("email")),
		FirstName:    strings.TrimSpace(r.PostForm.Get("first_name")),
		LastName:     strings.TrimSpace(r.PostForm.Get("last_name")),
		IsActive:     r.PostForm.Get("is_active") != "",
		IsSuperadmin: r.PostForm.Get("is_superadmin") != "",
	}
	password := r.PostForm.Get("password")

	user, err := a.app.Auth.CreateUser(auth.NewUser{
		Username:     form.Username,
		Email:        form.Email,
		FirstName:    form.FirstName,
		LastName:     form.LastName,
		Password:     password,
		IsSuperadmin: form.IsSuperadmin,
	})
	if err != nil {
		a.render(w, r, http.StatusBadRequest, "user_form.html", view.Data{
			"Nav": "users", "IsNew": true, "Form": form, "Error": humanize(err),
		})
		return
	}
	if !form.IsActive {
		user.IsActive = false
		if err := a.app.Auth.Users().Update(user); err != nil {
			a.render(w, r, http.StatusBadRequest, "user_form.html", view.Data{
				"Nav": "users", "IsNew": true, "Form": form, "Error": humanize(err),
			})
			return
		}
	}
	view.Flash(r, "success", "User "+user.Username+" created.")
	view.Redirect(w, r, a.prefix+"/users")
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
	user.FirstName = strings.TrimSpace(r.PostForm.Get("first_name"))
	user.LastName = strings.TrimSpace(r.PostForm.Get("last_name"))
	if !isSelf {
		user.IsActive = r.PostForm.Get("is_active") != ""
		user.IsSuperadmin = r.PostForm.Get("is_superadmin") != ""
	}

	fail := func(err error) {
		a.render(w, r, http.StatusBadRequest, "user_form.html", view.Data{
			"Nav": "users", "IsNew": false, "Form": user, "Error": humanize(err),
		})
	}

	if password := r.PostForm.Get("password"); password != "" {
		if err := a.app.Auth.ValidatePassword(password); err != nil {
			fail(err)
			return
		}
		hash, err := a.app.Auth.HashPassword(password)
		if err != nil {
			fail(err)
			return
		}
		user.Password = hash
		a.revokeUserSessions(user.ID)
	}

	if err := a.app.Auth.Users().Update(user); err != nil {
		fail(err)
		return
	}
	view.Flash(r, "success", "User "+user.Username+" updated.")
	view.Redirect(w, r, a.prefix+"/users")
}

func (a *Admin) userDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	me := a.currentUser(r)
	if me != nil && me.ID == id {
		view.Flash(r, "error", "You cannot delete your own account.")
		view.Redirect(w, r, a.prefix+"/users")
		return
	}
	if err := a.app.Auth.Users().Delete(id); err != nil {
		view.Flash(r, "error", humanize(err))
		view.Redirect(w, r, a.prefix+"/users")
		return
	}
	a.revokeUserSessions(id)
	view.Flash(r, "success", "User deleted.")
	view.Redirect(w, r, a.prefix+"/users")
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
	store, ok := a.sessionStore()
	if !ok {
		a.render(w, r, http.StatusOK, "sessions.html", view.Data{
			"Nav":         "sessions",
			"Unsupported": true,
		})
		return
	}
	current := session.FromRequest(r)
	rows := []sessionRow{}
	for _, s := range store.All() {
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
	a.render(w, r, http.StatusOK, "sessions.html", view.Data{
		"Nav":      "sessions",
		"Sessions": rows,
	})
}

func (a *Admin) sessionRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	current := session.FromRequest(r)
	if current != nil && current.ID() == id {
		view.Flash(r, "error", "Use log out to end your own session.")
		view.Redirect(w, r, a.prefix+"/sessions")
		return
	}
	if store, ok := a.sessionStore(); ok {
		_ = store.Delete(id)
		view.Flash(r, "success", "Session revoked.")
	} else {
		view.Flash(r, "error", "The configured session store cannot revoke sessions.")
	}
	view.Redirect(w, r, a.prefix+"/sessions")
}

func (a *Admin) routeList(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "routes.html", view.Data{
		"Nav":    "routes",
		"Routes": a.app.Routes(),
	})
}

func (a *Admin) settingsView(w http.ResponseWriter, r *http.Request) {
	s := a.app.Settings
	secretKey := "not set"
	if s.SecretKey != "" {
		secretKey = "set, " + strconv.Itoa(len(s.SecretKey)) + " characters hidden"
		if s.SecretKeyGenerated() {
			secretKey += " (generated for this run)"
		}
	}
	allowedHosts := "any host (Debug, no AllowedHosts set)"
	if len(s.AllowedHosts) > 0 {
		allowedHosts = strings.Join(s.AllowedHosts, ", ")
	}
	groups := []settingGroup{
		{"Core", []settingRow{
			{"Debug", boolText(s.Debug)},
			{"SecretKey", secretKey},
			{"AllowedHosts", allowedHosts},
			{"BaseDir", s.BaseDir},
		}},
	}
	for i, db := range s.Databases {
		redacted := db.Redacted()
		name := "Databases[" + strconv.Itoa(i) + "] " + redacted.Alias
		if i == 0 {
			name += " (default)"
		}
		rows := []settingRow{
			{"Engine", string(redacted.Engine)},
			{"Name", redacted.Name},
		}
		if !redacted.IsSQLite() {
			rows = append(rows,
				settingRow{"Host", redacted.Host},
				settingRow{"Port", strconv.Itoa(redacted.Port)},
				settingRow{"User", orDash(redacted.User)},
				settingRow{"Password", orDash(redacted.Password)},
			)
		}
		rows = append(rows,
			settingRow{"MaxOpenConns", poolText(redacted.MaxOpenConns)},
			settingRow{"MaxIdleConns", poolText(redacted.MaxIdleConns)},
			settingRow{"ConnMaxLifetime", durationText(redacted.ConnMaxLifetime)},
			settingRow{"ConnMaxIdleTime", durationText(redacted.ConnMaxIdleTime)},
			settingRow{"DSN", redacted.DSN()},
		)
		groups = append(groups, settingGroup{name, rows})
	}

	a.render(w, r, http.StatusOK, "settings.html", view.Data{
		"Nav": "settings",
		"Groups": append(groups, []settingGroup{
			{"Server", []settingRow{
				{"Addr", s.Addr()},
				{"ReadTimeout", durationText(s.Server.ReadTimeout)},
				{"WriteTimeout", durationText(s.Server.WriteTimeout)},
				{"IdleTimeout", durationText(s.Server.IdleTimeout)},
				{"ReadHeaderTimeout", durationText(s.Server.ReadHeaderTimeout)},
				{"ShutdownTimeout", durationText(s.Server.ShutdownTimeout)},
			}},
			{"Sessions", []settingRow{
				{"CookieName", s.Sessions.CookieName},
				{"Lifetime", s.Sessions.Lifetime.String()},
				{"Rolling", boolText(s.Sessions.Rolling)},
				{"Secure", boolText(s.Sessions.Secure)},
				{"HTTPOnly", boolText(s.Sessions.HTTPOnly)},
				{"SameSite", string(s.Sessions.SameSite)},
				{"Path", s.Sessions.Path},
				{"Domain", orDash(s.Sessions.Domain)},
				{"CleanupInterval", durationText(s.Sessions.CleanupInterval)},
				{"Store", storeName(a.app.SessionStore())},
			}},
			{"Auth", []settingRow{
				{"LoginURL", orDash(s.Auth.LoginURL)},
				{"PasswordMinLength", strconv.Itoa(s.Auth.PasswordMinLength)},
				{"PBKDF2Iterations", strconv.Itoa(s.Auth.PBKDF2Iterations)},
				{"UserStore", storeName(a.app.Auth.Users())},
			}},
			{"Templates", []settingRow{
				{"Layout", s.Templates.Layout},
				{"Shared", strings.Join(s.Templates.Shared, ", ")},
				{"Source", templateSource(s)},
				{"AutoReload", boolText(s.AutoReloadTemplates())},
			}},
			{"Static", []settingRow{
				{"URL", s.Static.URL},
				{"Source", staticSource(s)},
			}},
			{"Admin", []settingRow{
				{"Prefix", s.Admin.Prefix},
				{"SiteName", s.Admin.SiteName},
				{"Tagline", orDash(s.Admin.Tagline)},
			}},
			{"Logging", []settingRow{
				{"Level", s.Logging.Level},
				{"Format", s.Logging.Format},
			}},
		}...),
	})
}

func (a *Admin) notFound(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusNotFound, "notfound.html", view.Data{"Nav": ""})
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
	case errors.Is(err, auth.ErrLastSuperadmin):
		return "You cannot remove or disable the last active superadmin."
	case errors.Is(err, auth.ErrEmailExists):
		return "That email address is already registered."
	case errors.Is(err, auth.ErrInvalidEmail):
		return "That does not look like a valid email address."
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

type settingRow struct {
	Name  string
	Value string
}

type settingGroup struct {
	Name string
	Rows []settingRow
}

func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func poolText(n int) string {
	if n == 0 {
		return "driver default"
	}
	return strconv.Itoa(n)
}

func durationText(d time.Duration) string {
	if d == 0 {
		return "none"
	}
	return d.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func storeName(v any) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%T", v)
}

func templateSource(s settings.Settings) string {
	switch {
	case s.Templates.Dir != "":
		return "Dir " + s.Templates.Dir
	case s.Templates.FS != nil:
		return fmt.Sprintf("FS %T", s.Templates.FS)
	default:
		return "not configured"
	}
}

func staticSource(s settings.Settings) string {
	switch {
	case s.Static.Dir != "":
		return "Dir " + s.Static.Dir
	case s.Static.FS != nil:
		return fmt.Sprintf("FS %T", s.Static.FS)
	default:
		return "not configured"
	}
}
