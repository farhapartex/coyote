package accounts

import (
	"embed"
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/template"
	"github.com/farhapartex/coyote/core/view"
)

//go:embed templates locales
var templateFS embed.FS

type Options struct {
	Prefix            string
	AllowRegistration bool
	AfterLogin        string
	AfterLogout       string
	Pages             Pages
}

type Pages struct {
	Login    string
	Register string
	Profile  string
	Password string
}

type Accounts struct {
	app       *app.App
	prefix    string
	allowNew  bool
	afterIn   string
	afterOut  string
	pages     Pages
	templates *template.Engine
	router    *app.Router
}

func Mount(application *app.App, opts Options) *Accounts {
	prefix := "/" + strings.Trim(opts.Prefix, "/")
	if prefix == "/" {
		prefix = "/accounts"
	}
	if opts.AfterLogin == "" {
		opts.AfterLogin = "/"
	}
	if opts.AfterLogout == "" {
		opts.AfterLogout = "/"
	}

	a := &Accounts{
		app:      application,
		prefix:   prefix,
		allowNew: opts.AllowRegistration,
		afterIn:  opts.AfterLogin,
		afterOut: opts.AfterLogout,
		pages:    opts.Pages,
		templates: template.New(template.Options{
			FS:     templateFS,
			Layout: "base.html",
			Shared: []string{"templates/base.html"},
			Reload: application.Settings.AutoReloadTemplates(),
		}),
	}

	group := application.Group(prefix)
	group.Use(application.CSRF)
	a.router = group

	group.Get("/login", a.loginForm)
	group.Post("/login", a.loginSubmit)
	group.Post("/logout", a.logout)

	if a.allowNew {
		group.Get("/register", a.registerForm)
		group.Post("/register", a.registerSubmit)
	}

	i18n.Layer(application.Bundle(), templateFS, "locales")

	guarded := group.Group("", application.Auth.RequireLogin(prefix+"/login"))
	guarded.Get("/profile", a.profileForm)
	guarded.Post("/profile", a.profileSave)
	if application.Auth.AllowsPasswordChange() {
		guarded.Get("/password", a.passwordForm)
		guarded.Post("/password", a.passwordSave)
	}

	return a
}

func (a *Accounts) Prefix() string { return a.prefix }

func (a *Accounts) LoginURL() string { return a.prefix + "/login" }

func (a *Accounts) render(w http.ResponseWriter, r *http.Request, status int, page, override string, data view.Data) {
	if data == nil {
		data = view.Data{}
	}
	data["Prefix"] = a.prefix
	data["AllowRegistration"] = a.allowNew
	data["AllowPasswordChange"] = a.app.Auth.AllowsPasswordChange()

	if override != "" {
		a.app.RenderStatus(w, r, status, override, data)
		return
	}

	data = a.app.Context(r, data)
	if err := a.templates.Render(w, status, "templates/"+page, data); err != nil {
		a.app.Logger.Error("accounts render failed", "page", page, "error", err)
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}
