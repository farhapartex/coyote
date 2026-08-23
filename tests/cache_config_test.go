package tests

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/settings"
)

func TestCacheDefaultsToOneMemoryCache(t *testing.T) {
	resolved := settings.Default()
	if len(resolved.Caches) != 1 {
		t.Fatalf("Caches = %d entries, want 1", len(resolved.Caches))
	}

	entry := resolved.Cache()
	if entry.Alias != "default" {
		t.Errorf("Alias = %q, want %q", entry.Alias, "default")
	}
	if !entry.IsMemory() {
		t.Errorf("Backend = %q, want memory", entry.Backend)
	}
	if entry.TTL != cache.DefaultTTL {
		t.Errorf("TTL = %v, want %v", entry.TTL, cache.DefaultTTL)
	}
	if entry.Prefix != cache.DefaultPrefix {
		t.Errorf("Prefix = %q, want %q", entry.Prefix, cache.DefaultPrefix)
	}
}

func TestCacheAliasesAreFilledInOrder(t *testing.T) {
	resolved, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Caches = []settings.Cache{
			{Backend: settings.CacheInMemory},
			{Backend: settings.CacheInMemory},
			{Alias: "pages", Backend: settings.CacheInMemory},
		}
	})...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := []string{"default", "cache1", "pages"}
	for i, alias := range want {
		if resolved.Caches[i].Alias != alias {
			t.Errorf("Caches[%d].Alias = %q, want %q", i, resolved.Caches[i].Alias, alias)
		}
	}
	if _, found := resolved.CacheByAlias("pages"); !found {
		t.Error("CacheByAlias should find a named cache")
	}
	if _, found := resolved.CacheByAlias("nope"); found {
		t.Error("CacheByAlias should not invent a cache")
	}
}

func TestCacheBackendDefaultsAreApplied(t *testing.T) {
	resolved, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.BaseDir = "/srv/app"
		s.Caches = []settings.Cache{
			{Backend: settings.CacheInRedis},
			{Alias: "files", Backend: settings.CacheInFile},
		}
	})...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	redis := resolved.Caches[0]
	if redis.Address != settings.DefaultRedisAddress {
		t.Errorf("Address = %q, want %q", redis.Address, settings.DefaultRedisAddress)
	}
	if redis.DialTimeout != cache.DefaultDialTimeout {
		t.Errorf("DialTimeout = %v, want %v", redis.DialTimeout, cache.DefaultDialTimeout)
	}

	files := resolved.Caches[1]
	if files.Dir != "/srv/app/cache" {
		t.Errorf("Dir = %q, want a path under BaseDir", files.Dir)
	}
}

func TestCacheFileDirKeepsAnAbsolutePath(t *testing.T) {
	resolved, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Caches = []settings.Cache{{Backend: settings.CacheInFile, Dir: "/var/cache/coyote"}}
	})...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if resolved.Cache().Dir != "/var/cache/coyote" {
		t.Errorf("Dir = %q, want it left alone", resolved.Cache().Dir)
	}
}

func TestCacheValidationRejectsBadConfigurations(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*settings.Settings)
		want   string
	}{
		"unknown backend": {
			mutate: func(s *settings.Settings) {
				s.Caches = []settings.Cache{{Backend: "memcached"}}
			},
			want: "not supported",
		},
		"duplicate alias": {
			mutate: func(s *settings.Settings) {
				s.Caches = []settings.Cache{
					{Alias: "same", Backend: settings.CacheInMemory},
					{Alias: "same", Backend: settings.CacheInMemory},
				}
			},
			want: "reuses the alias",
		},
		"credentials on a memory cache": {
			mutate: func(s *settings.Settings) {
				s.Caches = []settings.Cache{{Backend: settings.CacheInMemory, Password: "hunter2"}}
			},
			want: "not used",
		},
		"tls on a file cache": {
			mutate: func(s *settings.Settings) {
				s.Caches = []settings.Cache{{Backend: settings.CacheInFile, Dir: "cache", TLS: &tls.Config{}}}
			},
			want: "TLS is not used",
		},
		"negative ttl": {
			mutate: func(s *settings.Settings) {
				s.Caches = []settings.Cache{{Backend: settings.CacheInMemory, TTL: -time.Minute}}
			},
			want: "TTL cannot be negative",
		},
		"store alongside redis": {
			mutate: func(s *settings.Settings) {
				s.Caches = []settings.Cache{{
					Backend: settings.CacheInRedis,
					Address: "127.0.0.1:6379",
					Store:   cache.NewNoopStore(),
				}}
			},
			want: "choose one",
		},
		"no caches at all": {
			mutate: func(s *settings.Settings) { s.Caches = []settings.Cache{} },
			want:   "Caches is empty",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := settings.New(append(prodSettings(), tc.mutate)...)
			if err == nil {
				t.Fatalf("expected a configuration error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestCacheRedactsThePassword(t *testing.T) {
	entry := settings.Cache{Backend: settings.CacheInRedis, Address: "10.0.0.5:6379", Password: "hunter2"}
	if redacted := entry.Redacted(); strings.Contains(redacted.Password, "hunter2") {
		t.Errorf("Redacted kept the password: %q", redacted.Password)
	}
	if entry.Password != "hunter2" {
		t.Error("Redacted must not mutate the original")
	}
	if entry.Location() != "10.0.0.5:6379" {
		t.Errorf("Location = %q", entry.Location())
	}
}

func TestCacheConstructorsProduceValidSettings(t *testing.T) {
	if entry := settings.MemoryCache(""); entry.Alias != "default" || !entry.IsMemory() {
		t.Errorf("MemoryCache = %+v", entry)
	}
	entry := settings.RedisCache("sessions", "")
	if entry.Alias != "sessions" || !entry.IsRedis() || entry.Address != settings.DefaultRedisAddress {
		t.Errorf("RedisCache = %+v", entry)
	}
}

func TestAppCacheIsUsableWithNoConfiguration(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()

	c := a.Cache()
	if c == nil {
		t.Fatal("Cache() must never return nil")
	}
	if err := c.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, found, err := c.Get(ctx, "key")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(value) != "value" {
		t.Errorf("value = %q", value)
	}
}

func TestAppCacheReturnsTheSameHandleEachTime(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()

	if err := a.Cache().Set(ctx, "shared", []byte("1"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := a.Cache().Get(ctx, "shared"); !found {
		t.Error("a second Cache() call should see the first call's write")
	}
}

func TestAppCacheByAliasResolvesAndRejects(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{
			{Backend: settings.CacheInMemory},
			{Alias: "pages", Backend: settings.CacheInMemory},
		}
	})
	ctx := context.Background()

	pages, err := a.CacheByAlias("pages")
	if err != nil {
		t.Fatalf("CacheByAlias: %v", err)
	}
	if err := pages.Set(ctx, "home", []byte("html"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := a.Cache().Get(ctx, "home"); found {
		t.Error("the default cache should not see another alias's key")
	}

	if _, err := a.CacheByAlias("nope"); err == nil {
		t.Error("an unknown alias should be an error")
	}
}

func TestAppCacheStatsReportBuiltCaches(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()

	if stats := a.CacheStats(); len(stats) != 0 {
		t.Errorf("CacheStats before use = %d entries, want 0", len(stats))
	}

	c := a.Cache()
	if err := c.Set(ctx, "key", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	c.Get(ctx, "key")
	c.Get(ctx, "absent")

	stats := a.CacheStats()
	if len(stats) != 1 {
		t.Fatalf("CacheStats = %d entries, want 1", len(stats))
	}
	if stats[0].Alias != "default" || stats[0].Backend != "memory" {
		t.Errorf("stats identity = %+v", stats[0])
	}
	if stats[0].Hits != 1 || stats[0].Misses != 1 {
		t.Errorf("stats = %+v", stats[0])
	}
}

func TestAppProbeCachesReportsMemoryAsReachable(t *testing.T) {
	a := newTestApp(t)
	statuses := a.ProbeCaches(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("ProbeCaches = %d entries, want 1", len(statuses))
	}
	if !statuses[0].Reachable || statuses[0].Err != nil {
		t.Errorf("an in-memory cache should always be reachable: %+v", statuses[0])
	}
}

func TestAppUsesASuppliedCacheStore(t *testing.T) {
	supplied := cache.NewMemoryStore(cache.MemoryOptions{})
	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{Backend: settings.CacheInMemory, Store: supplied, Prefix: "own"}}
	})
	ctx := context.Background()

	if err := a.Cache().Set(ctx, "key", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := supplied.Get(ctx, "own:key"); !found {
		t.Error("the supplied store should hold the namespaced key")
	}
}
