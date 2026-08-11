package main

import (
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/farhapartex/coyote/settings"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

func init() {
	templates, err := fs.Sub(templateFS, "templates")
	if err != nil {
		log.Fatal(err)
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}

	settings.Configure(func(s *settings.Settings) {
		s.Debug = settings.EnvBool("DEBUG", true)
		s.SecretKey = settings.Env("SECRET_KEY", "development-only-key-do-not-ship-this-value")
		s.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", []string{"127.0.0.1", "localhost"})

		s.Server.Host = settings.Env("HOST", "127.0.0.1")
		s.Server.Port = settings.EnvInt("PORT", 8000)

		s.Databases = []settings.Database{
			{
				Alias:           "default",
				Engine:          settings.SQLite,
				Name:            settings.Env("DB_NAME", "coyote.db"),
				MaxIdleConns:    2,
				ConnMaxIdleTime: 5 * time.Minute,
			},
		}

		s.Sessions.Lifetime = 8 * time.Hour
		s.Sessions.Rolling = true

		s.Templates.FS = templates
		s.Templates.Layout = "layouts/base.html"

		s.Static.FS = static
		s.Static.URL = "/static/"

		s.Admin.Prefix = "/admin"
		s.Admin.SiteName = "Coyote demo"
		s.Admin.Tagline = "session framework preview"

		s.Logging.Level = "debug"
	})
}
