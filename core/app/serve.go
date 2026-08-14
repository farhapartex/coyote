package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/certs"
)

func (a *App) Run() error {
	return cli.Dispatch(a)
}

func (a *App) Serve() error {
	s := a.Settings
	a.server = &http.Server{
		Addr:              s.Addr(),
		Handler:           a.Handler(),
		ReadHeaderTimeout: s.Server.ReadHeaderTimeout,
		ReadTimeout:       s.Server.ReadTimeout,
		WriteTimeout:      s.Server.WriteTimeout,
		IdleTimeout:       s.Server.IdleTimeout,
	}
	if s.Server.TLS.Enabled() {
		config, err := certs.TLSConfig(s.Server.TLS, s.AllowedHosts)
		if err != nil {
			return err
		}
		a.server.TLSConfig = config
	}
	if s.Server.Configure != nil {
		s.Server.Configure(a.server)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		a.Logger.Info("coyote listening",
			slog.String("url", s.BaseURL()),
			slog.String("version", Version),
			slog.String("environment", string(s.Environment)),
			slog.Bool("tls", s.Server.TLS.Enabled()),
			slog.Bool("autocert", s.Server.TLS.Managed()),
			slog.Bool("debug", s.Debug),
		)
		if err := a.listen(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	if err := a.CloseDB(); err != nil {
		a.Logger.Error("closing database", slog.Any("error", err))
	}
	return a.server.Shutdown(shutdownCtx)
}

func (a *App) listen() error {
	tls := a.Settings.Server.TLS
	switch {
	case !tls.Enabled():
		return a.server.ListenAndServe()
	case tls.Managed():
		return a.server.ListenAndServeTLS("", "")
	default:
		return a.server.ListenAndServeTLS(tls.CertFile, tls.KeyFile)
	}
}
