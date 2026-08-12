package settings

import (
	"html/template"
	"io/fs"
	"log/slog"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
)

type SameSite string

const (
	SameSiteLax    SameSite = "lax"
	SameSiteStrict SameSite = "strict"
	SameSiteNone   SameSite = "none"
)

type Settings struct {
	Debug        bool
	SecretKey    string
	AllowedHosts []string
	BaseDir      string

	Databases []Database

	Server     Server
	Migrations Migrations
	Sessions   Sessions
	Auth       Auth
	Templates  Templates
	Static     Static
	Admin      Admin
	Logging    Logging

	secretKeyGenerated bool
	commandOverrides   []string
}

type Migrations struct {
	Dir string
}

type Server struct {
	Host              string
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

type Sessions struct {
	CookieName      string
	Lifetime        time.Duration
	Rolling         bool
	Secure          bool
	HTTPOnly        bool
	SameSite        SameSite
	Path            string
	Domain          string
	CleanupInterval time.Duration
	Store           session.Store
}

type Auth struct {
	LoginURL          string
	PasswordMinLength int
	PBKDF2Iterations  int
	UserStore         auth.Store
}

type Templates struct {
	FS     fs.FS
	Dir    string
	Layout string
	Shared []string
	Funcs  template.FuncMap
}

type Static struct {
	URL string
	FS  fs.FS
	Dir string
}

type Admin struct {
	Prefix   string
	SiteName string
	Tagline  string
}

type Logging struct {
	Level  string
	Format string
	Logger *slog.Logger
}
