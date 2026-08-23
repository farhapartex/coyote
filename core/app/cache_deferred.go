package app

import (
	"context"
	"time"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/middleware"
)

type deferredCache struct {
	app   *App
	alias string
}

func fragmentCache(s Settings, a *App) cache.Cache {
	if len(s.Caches) == 0 {
		return nil
	}
	return deferredCache{app: a}
}

func (a *App) pageCache(s Settings) Middleware {
	if !s.PageCache.Active() {
		return nil
	}
	return middleware.PageCache(deferredCache{app: a, alias: s.PageCache.Alias}, s.PageCache)
}

func (d deferredCache) resolve() cache.Cache {
	if d.alias == "" {
		return d.app.Cache()
	}
	resolved, err := d.app.CacheByAlias(d.alias)
	if err != nil {
		d.app.Logger.Error("cache unavailable", "alias", d.alias, "error", err)
		return cache.New(cache.NewNoopStore(), cache.Options{Backend: "none"})
	}
	return resolved
}

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
