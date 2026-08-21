package settings

import (
	"crypto/tls"
	"time"

	"github.com/farhapartex/coyote/core/cache"
)

type CacheBackend string

const (
	CacheInMemory CacheBackend = "memory"
	CacheInRedis  CacheBackend = "redis"
	CacheInFile   CacheBackend = "file"
)

const (
	DefaultRedisAddress = "127.0.0.1:6379"
	DefaultCacheDir     = "cache"
)

type Cache struct {
	Alias   string
	Backend CacheBackend
	TTL     time.Duration
	Prefix  string
	Version string

	Address  string
	Username string
	Password string
	Database int
	TLS      *tls.Config

	Dir string

	MaxEntries      int
	MaxBytes        int64
	CleanupInterval time.Duration
	PoolSize        int
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration

	Store cache.Cache
}

func (c Cache) IsMemory() bool { return c.Backend == CacheInMemory }

func (c Cache) IsRedis() bool { return c.Backend == CacheInRedis }

func (c Cache) IsFile() bool { return c.Backend == CacheInFile }

func (c Cache) Location() string {
	switch c.Backend {
	case CacheInRedis:
		return c.Address
	case CacheInFile:
		return c.Dir
	default:
		return "in process"
	}
}

func (c Cache) Redacted() Cache {
	copied := c
	if copied.Password != "" {
		copied.Password = "••••••"
	}
	return copied
}

func MemoryCache(alias string) Cache {
	if alias == "" {
		alias = defaultAlias
	}
	return Cache{Alias: alias, Backend: CacheInMemory, TTL: cache.DefaultTTL, Prefix: cache.DefaultPrefix}
}

func RedisCache(alias, address string) Cache {
	if alias == "" {
		alias = defaultAlias
	}
	if address == "" {
		address = DefaultRedisAddress
	}
	return Cache{Alias: alias, Backend: CacheInRedis, Address: address, TTL: cache.DefaultTTL, Prefix: cache.DefaultPrefix}
}

func (s Settings) Cache() Cache {
	if len(s.Caches) == 0 {
		return Cache{}
	}
	return s.Caches[0]
}

func (s Settings) CacheByAlias(alias string) (Cache, bool) {
	for _, entry := range s.Caches {
		if entry.Alias == alias {
			return entry, true
		}
	}
	return Cache{}, false
}
