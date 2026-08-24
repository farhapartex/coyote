package admin

import (
	"embed"
	"strings"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/template"
)

//go:embed templates assets locales
var templateFS embed.FS

type Admin struct {
	app       *app.App
	prefix    string
	siteName  string
	tagline   string
	templates *template.Engine
	sections  []Section
	resources *resourceRegistry
	router    *app.Router
	guarded   *app.Router
}

func Mount(application *app.App) *Admin {
	cfg := application.Settings.Admin
	prefix := "/" + strings.Trim(cfg.Prefix, "/")

	a := &Admin{
		app:       application,
		prefix:    prefix,
		siteName:  cfg.SiteName,
		tagline:   cfg.Tagline,
		resources: newResourceRegistry(),
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

	a.routes(group, prefix+"/login")
	i18n.Layer(application.Bundle(), templateFS, "locales")

	return a
}

func (a *Admin) Prefix() string { return a.prefix }
