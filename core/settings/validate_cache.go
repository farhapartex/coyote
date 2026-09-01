package settings

import (
	"strconv"
	"strings"
)

func (s Settings) validateCaches(add func(string)) {
	if len(s.Caches) == 0 {
		add("Caches is empty; the first entry is the default cache, normally an in-memory one")
	}

	seen := make(map[string]bool, len(s.Caches))
	for i, entry := range s.Caches {
		label := "Caches[" + strconv.Itoa(i) + "]"

		if entry.Alias != "" {
			if seen[entry.Alias] {
				add(label + " reuses the alias " + strconv.Quote(entry.Alias))
			}
			seen[entry.Alias] = true
		}

		switch entry.Backend {
		case CacheInMemory:
			s.validateMemoryCache(label, entry, add)
		case CacheInRedis:
			s.validateRedisCache(label, entry, add)
		case CacheInFile:
			s.validateFileCache(label, entry, add)
		case "":
			add(label + ".Backend is empty")
		default:
			add(label + ".Backend " + strconv.Quote(string(entry.Backend)) +
				" is not supported; use \"memory\", \"redis\" or \"file\"")
		}

		if entry.TTL < 0 {
			add(label + ".TTL cannot be negative; use cache.Forever on a single call to skip expiry")
		}
		if entry.MaxEntries < 0 {
			add(label + ".MaxEntries cannot be negative")
		}
		if entry.MaxBytes < 0 {
			add(label + ".MaxBytes cannot be negative")
		}
		if entry.CleanupInterval < 0 {
			add(label + ".CleanupInterval cannot be negative")
		}
		if entry.Store != nil && entry.Backend != CacheInMemory {
			add(label + ".Store is set alongside the " + strconv.Quote(string(entry.Backend)) +
				" backend; choose one")
		}
	}
}

func (s Settings) validatePageCache(add func(string)) {
	page := s.PageCache
	if !page.Enabled {
		if page.TTL > 0 || page.Alias != "" || len(page.Paths) > 0 {
			add("PageCache is configured but PageCache.Enabled is false, so no page would be cached")
		}
		return
	}
	if page.TTL <= 0 {
		add("PageCache.Enabled is set but PageCache.TTL is not; a page cache with no lifetime caches nothing")
	}
	if len(page.Paths) == 0 {
		add("PageCache.Enabled is set but PageCache.Paths is empty; list the path prefixes to cache, " +
			"because caching every path would reach pages meant for one visitor")
	}
	if page.Alias != "" {
		if _, found := s.CacheByAlias(page.Alias); !found {
			add("PageCache.Alias " + strconv.Quote(page.Alias) + " does not match any entry in Caches")
		}
	}
	for _, prefix := range append(append([]string{}, page.Paths...), page.Skip...) {
		if !strings.HasPrefix(prefix, "/") {
			add("PageCache paths must start with \"/\"; " + strconv.Quote(prefix) + " does not")
		}
	}
}

func (s Settings) validateMemoryCache(label string, entry Cache, add func(string)) {
	if entry.Address != "" || entry.Username != "" || entry.Password != "" {
		add(label + " is an in-memory cache, so Address, Username and Password are not used")
	}
	if entry.Dir != "" {
		add(label + " is an in-memory cache, so Dir is not used")
	}
	if entry.TLS != nil {
		add(label + " is an in-memory cache, so TLS is not used")
	}
}

func (s Settings) validateRedisCache(label string, entry Cache, add func(string)) {
	if entry.Address == "" {
		add(label + ".Address is empty; it is the host:port Redis listens on")
	}
	if entry.Dir != "" {
		add(label + " is a Redis cache, so Dir is not used")
	}
	if entry.Database < 0 {
		add(label + ".Database cannot be negative")
	}
	if entry.PoolSize < 0 {
		add(label + ".PoolSize cannot be negative")
	}
	if entry.DialTimeout < 0 || entry.ReadTimeout < 0 || entry.WriteTimeout < 0 {
		add(label + " timeouts cannot be negative")
	}
}

func (s Settings) validateFileCache(label string, entry Cache, add func(string)) {
	if entry.Dir == "" {
		add(label + ".Dir is empty; it is the directory cached entries are written to")
	}
	if entry.Address != "" || entry.Username != "" || entry.Password != "" {
		add(label + " is a file cache, so Address, Username and Password are not used")
	}
	if entry.TLS != nil {
		add(label + " is a file cache, so TLS is not used")
	}
}
