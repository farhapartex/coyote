package settings

import (
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

const defaultAlias = "default"

func Default() Settings {
	return Settings{
		Debug:        false,
		Environment:  Development,
		AllowedHosts: nil,
		BaseDir:      workingDir(),
		Databases: []Database{
			{Alias: defaultAlias, Engine: SQLite, Name: DefaultSQLiteName},
		},
		Caches: []Cache{
			{
				Alias:           defaultAlias,
				Backend:         CacheInMemory,
				TTL:             cache.DefaultTTL,
				Prefix:          cache.DefaultPrefix,
				MaxEntries:      cache.DefaultMaxEntries,
				CleanupInterval: cache.DefaultCleanup,
			},
		},
		Security: Security{
			CSRF: true,
		},
		Server: Server{
			Host:              "127.0.0.1",
			Port:              8000,
			MaxBodyBytes:      32 << 20,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			ShutdownTimeout:   10 * time.Second,
		},
		Migrations: Migrations{
			Dir: "migrations",
		},
		Sessions: Sessions{
			CookieName:      "coyote_session",
			Lifetime:        12 * time.Hour,
			Rolling:         false,
			Secure:          false,
			HTTPOnly:        true,
			SameSite:        SameSiteLax,
			Path:            "/",
			CleanupInterval: 5 * time.Minute,
		},
		Auth: Auth{
			Permissions:         true,
			AllowPasswordChange: true,
			ResetTokenLifetime:  time.Hour,
			LoginURL:            "/admin/login",
			PasswordMinLength:   8,
			PBKDF2Iterations:    600000,
			Throttle: auth.ThrottlePolicy{
				Enabled:     true,
				MaxAttempts: 5,
				Window:      15 * time.Minute,
				Lockout:     15 * time.Minute,
			},
		},
		I18N: I18N{
			Default:   i18n.DefaultTag,
			Supported: []string{i18n.DefaultTag},
			Dir:       i18n.DefaultDir,
		},
		TimeZone: "UTC",
		Templates: Templates{
			Layout: "layouts/base.html",
			Shared: []string{"layouts/*.html", "partials/*.html"},
		},
		Pagination: Pagination{
			PerPage: view.DefaultPerPage,
		},
		Uploads: Uploads{
			Enabled:   false,
			Dir:       "media",
			MaxSize:   10 << 20,
			MaxPixels: 50_000_000,
			Allowed:   []string{"image/jpeg", "image/png", "image/gif", "application/pdf"},
			Serve:     false,
			URL:       "/media/",
			StageTTL:  24 * time.Hour,
			TrashTTL:  0,
		},
		Static: Static{
			URL: "/static/",
		},
		Admin: Admin{
			Prefix:   "/admin",
			SiteName: "Coyote administration",
		},
		Logging: Logging{
			Level:  "info",
			Format: "text",
		},
	}
}
