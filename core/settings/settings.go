package settings

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/storage"
	"github.com/farhapartex/coyote/core/view"
)

type SameSite string

const (
	SameSiteLax    SameSite = "lax"
	SameSiteStrict SameSite = "strict"
	SameSiteNone   SameSite = "none"
)

type Settings struct {
	Debug        bool
	Environment  Profile
	SecretKey    string
	AllowedHosts []string
	BaseDir      string

	Databases []Database

	Server     Server
	Security   Security
	Migrations Migrations
	Sessions   Sessions
	Auth       Auth
	Templates  Templates
	Pagination Pagination
	Uploads    Uploads
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
	TLS               TLS
	Configure         func(*http.Server)
}

type Sessions struct {
	Backend         SessionBackend
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

type Uploads struct {
	Enabled   bool
	Dir       string
	Path      string
	MaxSize   int64
	Allowed   []string
	MaxPixels int
	Serve     bool
	URL       string
	Private   bool
	StageTTL  time.Duration
	TrashTTL  time.Duration
	Storage   storage.Storage
}

type Pagination struct {
	PerPage   int
	Paginator view.Paginator
}

type Auth struct {
	LoginURL          string
	PasswordMinLength int
	PasswordRules     []auth.PasswordRule
	PBKDF2Iterations  int
	Throttle          auth.ThrottlePolicy
	Permissions       bool
	UserStore         auth.Store
	PermissionStore   auth.PermissionStore
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
