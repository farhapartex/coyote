package settings

import "time"

func Default() Settings {
	return Settings{
		Debug:        false,
		AllowedHosts: nil,
		BaseDir:      workingDir(),
		Databases: []Database{
			{Alias: "default", Engine: SQLite, Name: DefaultSQLiteName},
		},
		Server: Server{
			Host:              "127.0.0.1",
			Port:              8000,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			ShutdownTimeout:   10 * time.Second,
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
			LoginURL:          "/admin/login",
			PasswordMinLength: 8,
			PBKDF2Iterations:  600000,
		},
		Templates: Templates{
			Layout: "layouts/base.html",
			Shared: []string{"layouts/*.html", "partials/*.html"},
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
