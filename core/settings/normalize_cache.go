package settings

import (
	"path/filepath"
	"strconv"

	"github.com/farhapartex/coyote/core/cache"
)

func (s *Settings) normalizeCaches() {
	for i := range s.Caches {
		entry := &s.Caches[i]

		if entry.Alias == "" {
			if i == 0 {
				entry.Alias = defaultAlias
			} else {
				entry.Alias = "cache" + strconv.Itoa(i)
			}
		}
		if entry.Backend == "" {
			entry.Backend = CacheInMemory
		}
		if entry.Prefix == "" {
			entry.Prefix = cache.DefaultPrefix
		}
		if entry.TTL == 0 {
			entry.TTL = cache.DefaultTTL
		}

		switch entry.Backend {
		case CacheInMemory:
			if entry.MaxEntries <= 0 {
				entry.MaxEntries = cache.DefaultMaxEntries
			}
			if entry.CleanupInterval == 0 {
				entry.CleanupInterval = cache.DefaultCleanup
			}
		case CacheInRedis:
			if entry.Address == "" {
				entry.Address = DefaultRedisAddress
			}
			if entry.DialTimeout == 0 {
				entry.DialTimeout = cache.DefaultDialTimeout
			}
		case CacheInFile:
			if entry.Dir == "" {
				entry.Dir = DefaultCacheDir
			}
			if !filepath.IsAbs(entry.Dir) {
				entry.Dir = filepath.Join(s.BaseDir, entry.Dir)
			}
			if entry.CleanupInterval == 0 {
				entry.CleanupInterval = cache.DefaultCleanup
			}
		}
	}
}
