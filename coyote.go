package coyote

import (
	"context"
	"errors"
	"html/template"
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
)

const Version = "0.1.0"

type Config struct {
	Addr            string
	Templates       fs.FS
	Layout          string
	SharedTemplates []string
	TemplateFuncs   template.FuncMap
	DevMode         bool
	SessionName     string
	SessionLifetime time.Duration
	SessionSecure   bool
	SessionRolling  bool
	Logger          *slog.Logger
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type App struct {
	*Router
	Config    Config
	Logger    *slog.Logger
	Sessions  *session.Manager
	Auth      *auth.Service
	Templates *render.Engine
	Started   time.Time
	global    []Middleware
	server    *http.Server
	sessions  *session.MemoryStore
}

func New(cfg Config) *App {
	if cfg.Addr == "" {
		cfg.Addr = ":8000"
	}
	if cfg.Logger == nil {
		level := slog.LevelInfo
		if cfg.DevMode {
			level = slog.LevelDebug
		}
		cfg.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}
	if cfg.SessionLifetime <= 0 {
		cfg.SessionLifetime = 12 * time.Hour
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	store := session.NewMemoryStore(5 * time.Minute)
	sessions := session.NewManager(session.Options{
		Store:      store,
		CookieName: cfg.SessionName,
		Lifetime:   cfg.SessionLifetime,
		Rolling:    cfg.SessionRolling,
		Secure:     cfg.SessionSecure,
		HTTPOnly:   true,
		SameSite:   http.SameSiteLaxMode,
	})

	app := &App{
		Router:   newRouter(),
		Config:   cfg,
		Logger:   cfg.Logger,
		Sessions: sessions,
		Auth:     auth.NewService(auth.NewMemoryStore(), sessions),
		Templates: render.New(render.Options{
			FS:     cfg.Templates,
			Layout: cfg.Layout,
			Shared: cfg.SharedTemplates,
			Funcs:  cfg.TemplateFuncs,
			Reload: cfg.DevMode,
		}),
		Started:  time.Now(),
		sessions: store,
	}

	app.global = []Middleware{
		Recoverer(app.Logger),
		RequestLogger(app.Logger),
		SecureHeaders,
		sessions.Middleware,
		app.Auth.Middleware,
	}
	return app
}

func (a *App) Use(mw ...Middleware) {
	a.global = append(a.global, mw...)
}

func (a *App) SessionStore() *session.MemoryStore { return a.sessions }

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
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

func (a *App) Context(r *http.Request, data Data) Data {
	if data == nil {
		data = Data{}
	}
	sess := session.FromRequest(r)
	user := a.Auth.CurrentUser(r)
	setIfAbsent(data, "Request", r)
	setIfAbsent(data, "Path", r.URL.Path)
	setIfAbsent(data, "User", user)
	setIfAbsent(data, "Session", sess)
	setIfAbsent(data, "CSRFToken", a.Sessions.CSRFToken(r))
	setIfAbsent(data, "Version", Version)
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
	a.server = &http.Server{
		Addr:              a.Config.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       a.Config.ReadTimeout,
		WriteTimeout:      a.Config.WriteTimeout,
		IdleTimeout:       a.Config.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		a.Logger.Info("coyote listening",
			slog.String("addr", a.Config.Addr),
			slog.String("version", Version),
			slog.Bool("dev", a.Config.DevMode),
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.ShutdownTimeout)
	defer cancel()
	a.sessions.Close()
	return a.server.Shutdown(shutdownCtx)
}
