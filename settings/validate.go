package settings

import (
	"strconv"
	"strings"
)

const minSecretKeyLength = 32

const missingSettingsMessage = `no settings have been configured

Coyote needs an explicit settings file. Create settings.go next to your main package:

	package main

	import "github.com/farhapartex/coyote/settings"

	func init() {
		settings.Configure(func(s *settings.Settings) {
			s.Debug = true
			s.SecretKey = "replace-me-with-32-or-more-random-characters"
			s.AllowedHosts = []string{"127.0.0.1", "localhost"}
			s.Templates.Dir = "templates"
		})
	}

Every setting not assigned there keeps its default from settings.Default(), including a
SQLite database at <project>/coyote.db as Databases[0].`

type ImproperlyConfigured struct {
	Problems []string
}

func (e *ImproperlyConfigured) Error() string {
	if len(e.Problems) == 1 {
		return "coyote/settings: improperly configured: " + e.Problems[0]
	}
	var b strings.Builder
	b.WriteString("coyote/settings: improperly configured:")
	for _, p := range e.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p)
	}
	return b.String()
}

func (s Settings) validate() error {
	var problems []string
	add := func(format string) { problems = append(problems, format) }

	if s.SecretKey == "" {
		add("SecretKey is empty; set a random value of at least 32 characters (settings.GenerateSecretKey can make one)")
	} else if len(s.SecretKey) < minSecretKeyLength && !s.Debug {
		add("SecretKey is shorter than 32 characters")
	}

	if len(s.AllowedHosts) == 0 && !s.Debug {
		add("AllowedHosts is empty; with Debug disabled you must list the hosts this site serves, or use []string{\"*\"} to allow any")
	}
	for _, host := range s.AllowedHosts {
		if strings.TrimSpace(host) == "" {
			add("AllowedHosts contains an empty entry")
		}
	}

	if s.BaseDir == "" {
		add("BaseDir is empty")
	}

	if len(s.Databases) == 0 {
		add("Databases is empty; the first entry is the default connection, normally SQLite")
	}
	seenAliases := make(map[string]bool, len(s.Databases))
	for i, db := range s.Databases {
		label := "Databases[" + strconv.Itoa(i) + "]"
		if db.Alias != "" {
			if seenAliases[db.Alias] {
				add(label + " reuses the alias " + strconv.Quote(db.Alias))
			}
			seenAliases[db.Alias] = true
		}
		switch db.Engine {
		case SQLite:
			if db.Name == "" {
				add(label + ".Name is empty; it is the SQLite file path")
			}
			if db.Host != "" || db.User != "" || db.Password != "" {
				add(label + " is SQLite, so Host, User and Password are not used")
			}
		case Postgres, MySQL:
			if strings.TrimSpace(db.Name) == "" {
				add(label + ".Name is empty; it is the database name")
			}
			if strings.TrimSpace(db.Host) == "" {
				add(label + ".Host is empty")
			}
			if db.Port < 1 || db.Port > 65535 {
				add(label + ".Port must be between 1 and 65535")
			}
		case "":
			add(label + ".Engine is empty")
		default:
			add(label + ".Engine " + strconv.Quote(string(db.Engine)) + " is not supported; use \"sqlite\", \"postgres\" or \"mysql\"")
		}
		if db.MaxOpenConns < 0 {
			add(label + ".MaxOpenConns cannot be negative")
		}
		if db.MaxIdleConns < 0 {
			add(label + ".MaxIdleConns cannot be negative")
		}
		if db.ConnMaxLifetime < 0 {
			add(label + ".ConnMaxLifetime cannot be negative")
		}
		if db.ConnMaxIdleTime < 0 {
			add(label + ".ConnMaxIdleTime cannot be negative")
		}
	}

	if s.Server.Port < 1 || s.Server.Port > 65535 {
		add("Server.Port must be between 1 and 65535")
	}
	if s.Server.ShutdownTimeout < 0 {
		add("Server.ShutdownTimeout cannot be negative")
	}

	if s.Sessions.CookieName == "" {
		add("Sessions.CookieName is empty")
	}
	if strings.ContainsAny(s.Sessions.CookieName, " ;,\t\n") {
		add("Sessions.CookieName contains characters that are not valid in a cookie name")
	}
	if s.Sessions.Lifetime <= 0 {
		add("Sessions.Lifetime must be greater than zero")
	}
	switch s.Sessions.SameSite {
	case SameSiteLax, SameSiteStrict:
	case SameSiteNone:
		if !s.Sessions.Secure {
			add("Sessions.SameSite is \"none\", which browsers only accept when Sessions.Secure is true")
		}
	default:
		add("Sessions.SameSite must be \"lax\", \"strict\" or \"none\"")
	}
	if !strings.HasPrefix(s.Sessions.Path, "/") {
		add("Sessions.Path must start with \"/\"")
	}

	if s.Auth.PasswordMinLength < 6 {
		add("Auth.PasswordMinLength must be at least 6")
	}
	if s.Auth.PBKDF2Iterations < 1000 {
		add("Auth.PBKDF2Iterations must be at least 1000")
	}
	if s.Auth.LoginURL != "" && !strings.HasPrefix(s.Auth.LoginURL, "/") {
		add("Auth.LoginURL must start with \"/\"")
	}

	if s.Templates.Layout == "" {
		add("Templates.Layout is empty")
	}
	if s.Templates.FS != nil && s.Templates.Dir != "" {
		add("set either Templates.FS or Templates.Dir, not both")
	}

	if !strings.HasPrefix(s.Static.URL, "/") || !strings.HasSuffix(s.Static.URL, "/") {
		add("Static.URL must start and end with \"/\"")
	}
	if s.Static.FS != nil && s.Static.Dir != "" {
		add("set either Static.FS or Static.Dir, not both")
	}

	if !strings.HasPrefix(s.Admin.Prefix, "/") {
		add("Admin.Prefix must start with \"/\"")
	}
	if s.Admin.Prefix == "/" {
		add("Admin.Prefix cannot be \"/\", it would take over every route")
	}
	if strings.HasSuffix(s.Admin.Prefix, "/") && s.Admin.Prefix != "/" {
		add("Admin.Prefix must not end with \"/\"")
	}
	if s.Admin.SiteName == "" {
		add("Admin.SiteName is empty")
	}

	switch strings.ToLower(s.Logging.Level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		add("Logging.Level must be one of \"debug\", \"info\", \"warn\", \"error\"")
	}
	switch strings.ToLower(s.Logging.Format) {
	case "text", "json":
	default:
		add("Logging.Format must be \"text\" or \"json\"")
	}

	if len(problems) > 0 {
		return &ImproperlyConfigured{Problems: problems}
	}
	return nil
}
