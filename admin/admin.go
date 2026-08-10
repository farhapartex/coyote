package admin

import (
	"embed"
	"net/http"
	"sort"
	"strings"

	"github.com/farhapartex/coyote"
	"github.com/farhapartex/coyote/auth"
	"github.com/farhapartex/coyote/render"
	"github.com/farhapartex/coyote/session"
)

//go:embed templates
var templateFS embed.FS

type Options struct {
	Prefix   string
	SiteName string
	Tagline  string
}

type Section struct {
	Name        string
	Slug        string
	Description string
	Handler     http.Handler
}

type Admin struct {
	app       *coyote.App
	prefix    string
	siteName  string
	tagline   string
	templates *render.Engine
	sections  []Section
	router    *coyote.Router
}

func Mount(app *coyote.App, opts Options) *Admin {
	if opts.Prefix == "" {
		opts.Prefix = "/admin"
	}
	opts.Prefix = "/" + strings.Trim(opts.Prefix, "/")
	if opts.SiteName == "" {
		opts.SiteName = "Coyote administration"
	}

	a := &Admin{
		app:      app,
		prefix:   opts.Prefix,
		siteName: opts.SiteName,
		tagline:  opts.Tagline,
		templates: render.New(render.Options{
			FS:     templateFS,
			Layout: "base.html",
			Shared: []string{"templates/base.html"},
			Reload: app.Config.DevMode,
		}),
	}

	group := app.Group(opts.Prefix)
	group.Use(app.CSRF)
	a.router = group

	loginURL := opts.Prefix + "/login"
	group.Get("/login", a.loginForm)
	group.Post("/login", a.loginSubmit)

	guarded := group.Group("", app.Auth.RequireStaff(loginURL))
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
		guarded := a.router.Group("", a.app.Auth.RequireStaff(a.prefix+"/login"))
		guarded.Mount("/s/"+s.Slug, s.Handler)
	}
}

func (a *Admin) render(w http.ResponseWriter, r *http.Request, status int, page string, data coyote.Data) {
	if data == nil {
		data = coyote.Data{}
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

func (a *Admin) sessionStore() *session.MemoryStore {
	return a.app.SessionStore()
}
