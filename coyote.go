package coyote

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/farhapartex/coyote/auth"
	"github.com/farhapartex/coyote/render"
	"github.com/farhapartex/coyote/session"
	"github.com/farhapartex/coyote/settings"
)

const Version = "0.2.0"

type Settings = settings.Settings

type App struct {
	*Router
	Settings  Settings
	Logger    *slog.Logger
	Sessions  *session.Manager
	Auth      *auth.Service
	Templates *render.Engine
	Started   time.Time
	global    []Middleware
	server    *http.Server
	sessions  session.Store
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

	app := &App{
		Router:   newRouter(),
		Settings: s,
		Logger:   logger,
		Sessions: sessions,
		Auth: auth.NewService(s.Auth.UserStore, sessions, auth.Options{
			Hasher:            auth.Hasher{Iterations: s.Auth.PBKDF2Iterations},
			MinPasswordLength: s.Auth.PasswordMinLength,
		}),
		Templates: render.New(render.Options{
			FS:     templateFS(s),
			Layout: s.Templates.Layout,
			Shared: s.Templates.Shared,
			Funcs:  s.Templates.Funcs,
			Reload: s.AutoReloadTemplates(),
		}),
		Started:  time.Now(),
		sessions: store,
	}

	app.global = []Middleware{
		Recoverer(app.Logger),
		RequestLogger(app.Logger),
		AllowedHosts(s.AllowedHosts, s.Debug),
		SecureHeaders,
		sessions.Middleware,
		app.Auth.Middleware,
	}

	if fsys := staticFS(s); fsys != nil {
		app.Static(s.Static.URL, fsys)
	}

	if s.SecretKeyGenerated() {
		app.Logger.Warn("SecretKey was empty, generated an ephemeral development key; set SecretKey in settings.go before deploying")
	}
	if s.Debug {
		app.Logger.Warn("Debug is enabled; never run with Debug in production")
	}

	return app
}

func newLogger(s Settings) *slog.Logger {
	opts := &slog.HandlerOptions{Level: s.LogLevel()}
	if strings.EqualFold(s.Logging.Format, "json") {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func templateFS(s Settings) fs.FS {
	if s.Templates.FS != nil {
		return s.Templates.FS
	}
	if s.Templates.Dir != "" {
		return os.DirFS(s.Templates.Dir)
	}
	return nil
}

func staticFS(s Settings) fs.FS {
	if s.Static.FS != nil {
		return s.Static.FS
	}
	if s.Static.Dir != "" {
		return os.DirFS(s.Static.Dir)
	}
	return nil
}

func sameSite(mode settings.SameSite) http.SameSite {
	switch mode {
	case settings.SameSiteStrict:
		return http.SameSiteStrictMode
	case settings.SameSiteNone:
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func (a *App) SessionStore() session.Store { return a.sessions }

func (a *App) ManageableSessions() (session.ManageableStore, bool) {
	store, ok := a.sessions.(session.ManageableStore)
	return store, ok
}

func (a *App) Use(mw ...Middleware) {
	a.global = append(a.global, mw...)
}

func (a *App) Static(prefix string, fsys fs.FS) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	handler := http.StripPrefix(prefix, http.FileServerFS(fsys))
	a.Router.mux.Handle("GET "+prefix, handler)
	*a.Router.routes = append(*a.Router.routes, Route{Method: "GET", Pattern: prefix + "*"})
}

type Data map[string]any

func (a *App) Render(w http.ResponseWriter, r *http.Request, page string, data Data) {
	a.RenderStatus(w, r, http.StatusOK, page, data)
}

func (a *App) RenderStatus(w http.ResponseWriter, r *http.Request, status int, page string, data Data) {
	if data == nil {
		data = Data{}
	}
	a.Context(r, data)
	if err := a.Templates.Render(w, status, page, data); err != nil {
		a.Logger.Error("render failed", slog.String("page", page), slog.Any("error", err))
		if a.Settings.Debug {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

func (a *App) Context(r *http.Request, data Data) Data {
	if data == nil {
		data = Data{}
	}
	sess := session.FromRequest(r)
	setIfAbsent(data, "Request", r)
	setIfAbsent(data, "Path", r.URL.Path)
	setIfAbsent(data, "User", a.Auth.CurrentUser(r))
	setIfAbsent(data, "Session", sess)
	setIfAbsent(data, "CSRFToken", a.Sessions.CSRFToken(r))
	setIfAbsent(data, "Version", Version)
	setIfAbsent(data, "Debug", a.Settings.Debug)
	if sess != nil {
		setIfAbsent(data, "Flashes", sess.Flashes())
	}
	return data
}

func setIfAbsent(data Data, key string, value any) {
	if _, exists := data[key]; !exists {
		data[key] = value
	}
}

func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func Flash(r *http.Request, kind, message string) {
	if sess := session.FromRequest(r); sess != nil {
		sess.AddFlash(kind, message)
	}
}

func (a *App) Handler() http.Handler {
	return chain(a.Router.mux, a.global...)
}

func (a *App) Run() error {
	s := a.Settings
	a.server = &http.Server{
		Addr:              s.Addr(),
		Handler:           a.Handler(),
		ReadHeaderTimeout: s.Server.ReadHeaderTimeout,
		ReadTimeout:       s.Server.ReadTimeout,
		WriteTimeout:      s.Server.WriteTimeout,
		IdleTimeout:       s.Server.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		a.Logger.Info("coyote listening",
			slog.String("addr", s.Addr()),
			slog.String("version", Version),
			slog.Bool("debug", s.Debug),
		)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		a.Logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.Server.ShutdownTimeout)
	defer cancel()
	if closer, ok := a.sessions.(interface{ Close() }); ok {
		closer.Close()
	}
	return a.server.Shutdown(shutdownCtx)
}
