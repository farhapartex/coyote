package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/cache/redis"
	"github.com/farhapartex/coyote/core/settings"
)

type cacheEntry struct {
	once   sync.Once
	handle *cache.Handle
	err    error
}

func (a *App) Cache() cache.Cache {
	handle, err := a.cacheByAlias(a.Settings.Cache().Alias)
	if err != nil {
		a.Logger.Error("cache unavailable, falling back to no caching", "error", err)
		return cache.New(cache.NewNoopStore(), cache.Options{Backend: "none"})
	}
	return handle
}

func (a *App) CacheByAlias(alias string) (cache.Cache, error) {
	return a.cacheByAlias(alias)
}

func (a *App) cacheByAlias(alias string) (*cache.Handle, error) {
	if alias == "" {
		alias = a.Settings.Cache().Alias
	}
	cfg, found := a.Settings.CacheByAlias(alias)
	if !found {
		return nil, fmt.Errorf("coyote/app: no cache configured with alias %q", alias)
	}

	stored, _ := a.caches.LoadOrStore(alias, &cacheEntry{})
	entry := stored.(*cacheEntry)
	entry.once.Do(func() {
		store, err := buildCacheStore(cfg)
		if err != nil {
			entry.err = err
			return
		}
		entry.handle = cache.New(store, cache.Options{
			Alias:   cfg.Alias,
			Backend: string(cfg.Backend),
			Prefix:  cfg.Prefix,
			Version: cfg.Version,
			TTL:     cfg.TTL,
		})
	})
	return entry.handle, entry.err
}

func buildCacheStore(cfg settings.Cache) (cache.Cache, error) {
	if cfg.Store != nil {
		return cfg.Store, nil
	}
	switch cfg.Backend {
	case settings.CacheInMemory:
		return cache.NewMemoryStore(cache.MemoryOptions{
			MaxEntries:      cfg.MaxEntries,
			MaxBytes:        cfg.MaxBytes,
			CleanupInterval: cfg.CleanupInterval,
		}), nil
	case settings.CacheInFile:
		return cache.NewFileStore(cache.FileOptions{
			Dir:             cfg.Dir,
			CleanupInterval: cfg.CleanupInterval,
		}), nil
	case settings.CacheInRedis:
		return redis.New(redis.Options{
			Address:      cfg.Address,
			Username:     cfg.Username,
			Password:     cfg.Password,
			Database:     cfg.Database,
			TLS:          cfg.TLS,
			PoolSize:     cfg.PoolSize,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		}), nil
	default:
		return nil, fmt.Errorf("coyote/app: the %q cache backend is not available in this build", cfg.Backend)
	}
}

func (a *App) CacheStats() []cache.Stats {
	out := []cache.Stats{}
	for _, cfg := range a.Settings.Caches {
		stored, found := a.caches.Load(cfg.Alias)
		if !found {
			continue
		}
		if entry, ok := stored.(*cacheEntry); ok && entry.handle != nil {
			out = append(out, entry.handle.Stats())
		}
	}
	return out
}

func (a *App) ProbeCaches(ctx context.Context) []cache.Status {
	out := make([]cache.Status, 0, len(a.Settings.Caches))
	for _, cfg := range a.Settings.Caches {
		status := cache.Status{
			Alias:   cfg.Alias,
			Backend: string(cfg.Backend),
			Address: cfg.Redacted().Location(),
		}
		handle, err := a.cacheByAlias(cfg.Alias)
		if err != nil {
			status.Err = err
			out = append(out, status)
			continue
		}
		if err := handle.Ping(ctx); err != nil {
			status.Err = err
			out = append(out, status)
			continue
		}
		status.Reachable = true
		out = append(out, status)
	}
	return out
}

func (a *App) closeCaches() {
	a.caches.Range(func(_, value any) bool {
		if entry, ok := value.(*cacheEntry); ok && entry.handle != nil {
			if err := entry.handle.Close(); err != nil {
				a.Logger.Error("closing cache", "alias", entry.handle.Alias(), "error", err)
			}
		}
		return true
	})
}
