package settings

import (
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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

	Server    Server
	Sessions  Sessions
	Auth      Auth
	Templates Templates
	Static    Static
	Admin     Admin
	Logging   Logging

	secretKeyGenerated bool
}

type Engine string

const (
	SQLite   Engine = "sqlite"
	Postgres Engine = "postgres"
	MySQL    Engine = "mysql"
)

const DefaultSQLiteName = "coyote.db"

type Database struct {
	Alias           string
	Engine          Engine
	Name            string
	Host            string
	Port            int
	User            string
	Password        string
	Options         map[string]string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (d Database) IsSQLite() bool { return d.Engine == SQLite }

func (d Database) DSN() string {
	options := d.encodedOptions()
	switch d.Engine {
	case SQLite:
		if options == "" {
			return d.Name
		}
		return d.Name + "?" + options
	case Postgres:
		u := url.URL{
			Scheme:   "postgres",
			Host:     net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
			Path:     "/" + d.Name,
			RawQuery: options,
		}
		if d.User != "" {
			u.User = url.UserPassword(d.User, d.Password)
		}
		return u.String()
	case MySQL:
		credentials := d.User
		if d.Password != "" {
			credentials += ":" + d.Password
		}
		if credentials != "" {
			credentials += "@"
		}
		dsn := credentials + "tcp(" + net.JoinHostPort(d.Host, strconv.Itoa(d.Port)) + ")/" + d.Name
		if options != "" {
			dsn += "?" + options
		}
		return dsn
	default:
		return d.Name
	}
}

func (d Database) Redacted() Database {
	copied := d
	if copied.Password != "" {
		copied.Password = "••••••"
	}
	return copied
}

func (d Database) encodedOptions() string {
	if len(d.Options) == 0 {
		return ""
	}
	keys := make([]string, 0, len(d.Options))
	for k := range d.Options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := url.Values{}
	for _, k := range keys {
		values.Set(k, d.Options[k])
	}
	return values.Encode()
}

func SQLiteDatabase(alias, name string) Database {
	if alias == "" {
		alias = "default"
	}
	if name == "" {
		name = DefaultSQLiteName
	}
	return Database{Alias: alias, Engine: SQLite, Name: name}
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
	s.normalize()
	if err := s.validate(); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func (s *Settings) normalize() {
	if s.BaseDir == "" {
		s.BaseDir = workingDir()
	}
	if abs, err := filepath.Abs(s.BaseDir); err == nil {
		s.BaseDir = abs
	}

	for i := range s.Databases {
		db := &s.Databases[i]
		if db.Alias == "" {
			if i == 0 {
				db.Alias = "default"
			} else {
				db.Alias = "db" + strconv.Itoa(i)
			}
		}
		if db.Engine == "" {
			db.Engine = SQLite
		}
		switch db.Engine {
		case SQLite:
			if db.Name == "" {
				db.Name = DefaultSQLiteName
			}
			if db.Name != ":memory:" && !filepath.IsAbs(db.Name) {
				db.Name = filepath.Join(s.BaseDir, db.Name)
			}
		case Postgres:
			if db.Host == "" {
				db.Host = "127.0.0.1"
			}
			if db.Port == 0 {
				db.Port = 5432
			}
		case MySQL:
			if db.Host == "" {
				db.Host = "127.0.0.1"
			}
			if db.Port == 0 {
				db.Port = 3306
			}
		}
	}
}

func (s Settings) Database() Database {
	if len(s.Databases) == 0 {
		return Database{}
	}
	return s.Databases[0]
}

func (s Settings) DatabaseByAlias(alias string) (Database, bool) {
	for _, db := range s.Databases {
		if db.Alias == alias {
			return db, true
		}
	}
	return Database{}, false
}

func (s Settings) Path(elements ...string) string {
	return filepath.Join(append([]string{s.BaseDir}, elements...)...)
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
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
