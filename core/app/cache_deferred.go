package app

import (
	"context"
	"time"

	"github.com/farhapartex/coyote/core/cache"
)

type deferredCache struct {
	app *App
}

func fragmentCache(s Settings, a *App) cache.Cache {
	if len(s.Caches) == 0 {
		return nil
	}
	return deferredCache{app: a}
}

func (d deferredCache) resolve() cache.Cache { return d.app.Cache() }

func (d deferredCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return d.resolve().Get(ctx, key)
}

func (d deferredCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return d.resolve().Set(ctx, key, value, ttl)
}

func (d deferredCache) Delete(ctx context.Context, key string) error {
	return d.resolve().Delete(ctx, key)
}

func (d deferredCache) Has(ctx context.Context, key string) (bool, error) {
	return d.resolve().Has(ctx, key)
}

func (d deferredCache) Clear(ctx context.Context) error {
	return d.resolve().Clear(ctx)
}
