package main

import (
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
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

	settings.MustLoadDotEnv(".env")

	settings.Configure(settings.Preset(settings.Env("APP_ENV", "development")), func(s *settings.Settings) {
		s.SecretKey = settings.Env("SECRET_KEY", "development-only-key-do-not-ship-this-value")
		if hosts := settings.EnvList("ALLOWED_HOSTS", nil); hosts != nil {
			s.AllowedHosts = hosts
		}

		s.Server.Host = settings.Env("HOST", "127.0.0.1")
		s.Server.Port = settings.EnvInt("PORT", 8081)
		s.Server.TLS.CertFile = settings.Env("TLS_CERT", "")
		s.Server.TLS.KeyFile = settings.Env("TLS_KEY", "")

		s.Databases = []settings.Database{
			{
				Alias:           "default",
				Engine:          settings.SQLite,
				Name:            settings.Env("DB_NAME", "coyote.db"),
				MaxOpenConns:    8,
				MaxIdleConns:    4,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: 5 * time.Minute,
			},
		}

		s.Sessions.Lifetime = 8 * time.Hour
		s.Sessions.Rolling = true

		s.Auth.LoginURL = "/accounts/login"
		s.Auth.Throttle = auth.ThrottlePolicy{
			Enabled:     settings.EnvBool("LOGIN_THROTTLE", true),
			MaxAttempts: 5,
			Window:      10 * time.Minute,
			Lockout:     2 * time.Minute,
		}

		s.Uploads.Enabled = true
		s.Uploads.Serve = true
		s.Uploads.Path = "uploads"
		s.Uploads.MaxSize = 5 << 20
		s.Uploads.Allowed = []string{"image/png", "image/jpeg", "image/gif", "application/pdf"}

		s.Templates.FS = templates
		s.Templates.Layout = "layouts/base.html"

		s.Static.FS = static
		s.Static.URL = "/static/"

		s.Admin.Prefix = "/admin"
		s.Admin.SiteName = "Coyote demo"
		s.Admin.Tagline = "session framework preview"

	})
}
