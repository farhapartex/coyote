package app

import (
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/router"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/template"
	"github.com/farhapartex/coyote/core/view"
	"gorm.io/gorm"
)

const Version = "0.3.0"

type (
	Settings   = settings.Settings
	Middleware = router.Middleware
	Route      = router.Route
	Router     = router.Router
	Data       = view.Data
)

type App struct {
	*router.Router
	Settings  Settings
	Logger    *slog.Logger
	Sessions  *session.Manager
	Auth      *auth.Service
	Templates *template.Engine
	Started   time.Time
	global    []Middleware
	server    *http.Server
	sessions  session.Store
	models    *model.Registry
	dbOnce    sync.Once
	dbHandle  *gorm.DB
	dbErr     error
}

func New() *App {
	return NewFrom(settings.Get())
}

func NewFrom(s Settings) *App {
	logger := s.Logging.Logger
	if logger == nil {
		logger = newLogger(s)
	}

	store := s.Sessions.Store
	if store == nil {
		store = session.NewMemoryStore(s.Sessions.CleanupInterval)
	}

	sessions := session.NewManager(session.Options{
		Store:      store,
		CookieName: s.Sessions.CookieName,
		Lifetime:   s.Sessions.Lifetime,
		Rolling:    s.Sessions.Rolling,
		Secure:     s.Sessions.Secure,
		HTTPOnly:   s.Sessions.HTTPOnly,
		SameSite:   sameSite(s.Sessions.SameSite),
		Path:       s.Sessions.Path,
		Domain:     s.Sessions.Domain,
	})

	a := &App{
		Router:   router.New(),
		Settings: s,
		Logger:   logger,
		Sessions: sessions,
		Auth: auth.NewService(s.Auth.UserStore, sessions, auth.Options{
			Hasher:            auth.Hasher{Iterations: s.Auth.PBKDF2Iterations},
			MinPasswordLength: s.Auth.PasswordMinLength,
		}),
		Templates: template.New(template.Options{
			FS:     templateFS(s),
			Layout: s.Templates.Layout,
			Shared: s.Templates.Shared,
			Funcs:  s.Templates.Funcs,
			Reload: s.AutoReloadTemplates(),
		}),
		Started:  time.Now(),
		sessions: store,
		models:   defaultModels(),
	}

	a.global = []Middleware{
		middleware.Recoverer(a.Logger),
		middleware.RequestLogger(a.Logger),
		middleware.AllowedHosts(s.AllowedHosts, s.Debug),
		middleware.SecureHeaders,
		sessions.Middleware,
		a.Auth.Middleware,
	}

	if fsys := staticFS(s); fsys != nil {
		a.Router.Static(s.Static.URL, fsys)
	}

	if s.SecretKeyGenerated() {
		a.Logger.Warn("SecretKey was empty, generated an ephemeral development key; set SecretKey in settings.go before deploying")
	}
	if s.Debug {
		a.Logger.Warn("Debug is enabled; never run with Debug in production")
	}

	return a
}

func (a *App) Config() Settings { return a.Settings }

func (a *App) Log() *slog.Logger { return a.Logger }

func (a *App) SessionStore() session.Store { return a.sessions }

func (a *App) ManageableSessions() (session.ManageableStore, bool) {
	store, ok := a.sessions.(session.ManageableStore)
	return store, ok
}

func (a *App) Use(mw ...Middleware) {
	a.global = append(a.global, mw...)
}

func (a *App) CSRF(next http.Handler) http.Handler {
	return middleware.CSRF(a.Sessions)(next)
}

func (a *App) Static(prefix string, fsys fs.FS) {
	a.Router.Static(prefix, fsys)
}

func (a *App) Handler() http.Handler {
	return router.Chain(a.Router, a.global...)
}
