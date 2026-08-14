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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		a.Logger.Info("coyote listening",
			slog.String("addr", s.Addr()),
			slog.String("version", Version),
			slog.String("environment", string(s.Environment)),
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
	if err := a.CloseDB(); err != nil {
		a.Logger.Error("closing database", slog.Any("error", err))
	}
	return a.server.Shutdown(shutdownCtx)
}
