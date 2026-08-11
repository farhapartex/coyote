package admin

import (
	"embed"
	"net/http"
	"sort"
	"strings"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/template"
	"github.com/farhapartex/coyote/core/view"
)

//go:embed templates
var templateFS embed.FS

type Section struct {
	Name        string
	Slug        string
	Description string
	Handler     http.Handler
}

type Admin struct {
	app       *app.App
	prefix    string
	siteName  string
	tagline   string
	templates *template.Engine
	sections  []Section
	router    *app.Router
}

func Mount(app *app.App) *Admin {
	cfg := app.Settings.Admin
	prefix := "/" + strings.Trim(cfg.Prefix, "/")

	a := &Admin{
		app:      app,
		prefix:   prefix,
		siteName: cfg.SiteName,
		tagline:  cfg.Tagline,
		templates: template.New(template.Options{
			FS:     templateFS,
			Layout: "base.html",
			Shared: []string{"templates/base.html"},
			Reload: app.Settings.AutoReloadTemplates(),
		}),
	}

	group := app.Group(prefix)
	group.Use(app.CSRF)
	a.router = group

	loginURL := prefix + "/login"
	group.Get("/login", a.loginForm)
	group.Post("/login", a.loginSubmit)

	guarded := group.Group("", app.Auth.RequireSuperadmin(loginURL))
	guarded.Post("/logout", a.logout)
	guarded.Get("/", a.dashboard)
	guarded.Get("/users", a.userList)
	guarded.Get("/users/new", a.userForm)
	guarded.Post("/users/new", a.userCreate)
	guarded.Get("/users/{id}", a.userForm)
	guarded.Post("/users/{id}", a.userUpdate)
	guarded.Post("/users/{id}/delete", a.userDelete)
	guarded.Get("/sessions", a.sessionList)
	guarded.Post("/sessions/{id}/revoke", a.sessionRevoke)
	guarded.Get("/routes", a.routeList)
	guarded.Get("/settings", a.settingsView)

	return a
}

func (a *Admin) Prefix() string { return a.prefix }

func (a *Admin) Register(s Section) {
	if s.Slug == "" {
		s.Slug = strings.ToLower(strings.ReplaceAll(s.Name, " ", "-"))
	}
	a.sections = append(a.sections, s)
	sort.Slice(a.sections, func(i, j int) bool { return a.sections[i].Name < a.sections[j].Name })
	if s.Handler != nil {
		guarded := a.router.Group("", a.app.Auth.RequireSuperadmin(a.prefix+"/login"))
		guarded.Mount("/s/"+s.Slug, s.Handler)
	}
}

func (a *Admin) render(w http.ResponseWriter, r *http.Request, status int, page string, data view.Data) {
	if data == nil {
		data = view.Data{}
	}
	a.app.Context(r, data)
	data["Prefix"] = a.prefix
	data["SiteName"] = a.siteName
	data["Tagline"] = a.tagline
	data["Sections"] = a.sections
	if err := a.templates.Render(w, status, "templates/"+page, data); err != nil {
		a.app.Logger.Error("admin render failed: " + err.Error())
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

func (a *Admin) currentUser(r *http.Request) *auth.User {
	return a.app.Auth.CurrentUser(r)
}

func (a *Admin) sessionStore() (session.ManageableStore, bool) {
	return a.app.ManageableSessions()
}

func (a *Admin) sessionCount() int {
	if store, ok := a.sessionStore(); ok {
		return store.Count()
	}
	return -1
}

func (a *Admin) revokeUserSessions(userID string) {
	if store, ok := a.sessionStore(); ok {
		store.DeleteByUserID(userID)
	}
}
