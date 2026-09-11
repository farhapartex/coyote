package app

import (
	"context"
	"log/slog"
)

func (a *App) shutdown(ctx context.Context) error {
	drained := a.server.Shutdown(ctx)
	a.drainJobs()
	a.closeStores()
	return drained
}

func (a *App) closeStores() {
	if closer, ok := a.sessions.(interface{ Close() }); ok {
		closer.Close()
	}
	a.closeCaches()
	if err := a.CloseDB(); err != nil {
		a.Logger.Error("closing database", slog.Any("error", err))
	}
}
