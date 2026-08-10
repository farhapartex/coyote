package settings

import (
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/farhapartex/coyote/auth"
	"github.com/farhapartex/coyote/session"
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

	Server    Server
	Sessions  Sessions
	Auth      Auth
	Templates Templates
	Static    Static
	Admin     Admin
	Logging   Logging

	secretKeyGenerated bool
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

func Default() Settings {
	return Settings{
		Debug:        false,
		AllowedHosts: nil,
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

func New(fns ...func(*Settings)) (Settings, error) {
	s := Default()
	for _, fn := range fns {
		if fn != nil {
			fn(&s)
		}
	}
	if s.SecretKey == "" && s.Debug {
		key, err := generateSecretKey()
		if err != nil {
			return Settings{}, &ImproperlyConfigured{Problems: []string{
				"SecretKey is empty and a development key could not be generated: " + err.Error(),
			}}
		}
		s.SecretKey = key
		s.secretKeyGenerated = true
	}
	if err := s.validate(); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func (s Server) Addr() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func (s Settings) Addr() string { return s.Server.Addr() }

func (s Settings) SecretKeyGenerated() bool { return s.secretKeyGenerated }

func (s Settings) LogLevel() slog.Level {
	switch strings.ToLower(s.Logging.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (s Settings) AutoReloadTemplates() bool { return s.Debug }

var (
	mu         sync.RWMutex
	configured *Settings
)

func Configure(fns ...func(*Settings)) Settings {
	s, err := New(fns...)
	if err != nil {
		panic(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if configured != nil {
		panic(&ImproperlyConfigured{Problems: []string{
			"settings have already been configured; call settings.Configure exactly once, from settings.go",
		}})
	}
	configured = &s
	return s
}

func Get() Settings {
	mu.RLock()
	defer mu.RUnlock()
	if configured == nil {
		panic(&ImproperlyConfigured{Problems: []string{missingSettingsMessage}})
	}
	return *configured
}

func IsConfigured() bool {
	mu.RLock()
	defer mu.RUnlock()
	return configured != nil
}

func Reset() {
	mu.Lock()
	defer mu.Unlock()
	configured = nil
}

func generateSecretKey() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func GenerateSecretKey() (string, error) { return generateSecretKey() }
