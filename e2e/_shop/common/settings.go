package main

import (
	"embed"
	"io/fs"
	"time"

	"github.com/farhapartex/coyote/core/settings"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

func init() {
	templates, _ := fs.Sub(templateFS, "templates")
	static, _ := fs.Sub(staticFS, "static")

	settings.MustLoadDotEnv(".env")

	settings.Configure(
		settings.Preset(settings.Env("APP_ENV", "development")),
		func(s *settings.Settings) {
			s.SecretKey = settings.Env("SECRET_KEY", "")
			s.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", []string{"127.0.0.1", "localhost"})

			s.Server.Host = settings.Env("HOST", "127.0.0.1")
			s.Server.Port = settings.EnvInt("PORT", 8099)

			s.Databases = []settings.Database{{
				Alias:        "default",
				Engine:       settings.Postgres,
				Host:         settings.Env("DB_HOST", "127.0.0.1"),
				Port:         settings.EnvInt("DB_PORT", 55432),
				Name:         settings.Env("DB_NAME", "shop"),
				User:         settings.Env("DB_USER", "shop"),
				Password:     settings.Env("DB_PASSWORD", "shop"),
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			}}

			s.Caches = []settings.Cache{
				settings.RedisCache("default", settings.Env("REDIS_ADDR", "127.0.0.1:56379")),
			}

			s.Jobs.Enabled = true
			s.Jobs.Workers = settings.EnvInt("JOB_WORKERS", 2)
			s.Jobs.PollInterval = 200 * time.Millisecond
			s.Jobs.DoneTTL = time.Hour

			s.Uploads.Enabled = true
			s.Uploads.Serve = true
			s.Uploads.Dir = "media"
			s.Uploads.URL = "/media/"
			s.Uploads.Allowed = []string{"image/jpeg", "image/png", "image/gif"}
			s.Uploads.MaxSize = 4 << 20

			s.Templates.FS = templates
			s.Static.FS = static

			s.Pagination.PerPage = 12

			s.Auth.AllowPasswordChange = true

			s.Admin.SiteName = "Thornfield Supply"
			s.Admin.Tagline = "Inventory, orders and customers"
		},
	)
}
