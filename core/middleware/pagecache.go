package middleware

import (
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/settings"
)

const (
	CacheStatusHeader = "X-Cache"
	CacheHit          = "HIT"
	CacheMiss         = "MISS"

	pagePrefix = "page:"
	varyPrefix = "page:vary:"
)

func PageCache(c cache.Cache, policy settings.PageCache) Middleware {
	if c == nil || policy.TTL <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cacheableRequest(r, policy) {
				next.ServeHTTP(w, r)
				return
			}

			base := baseKey(r)
			if entry, found := lookupPage(r, c, base); found {
				entry.writeTo(w)
				return
			}

			w.Header().Set(CacheStatusHeader, CacheMiss)
			recorder := &pageRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			storePage(r, c, base, recorder, policy)
		})
	}
}

func cacheableRequest(r *http.Request, policy settings.PageCache) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	if hasDirective(r.Header, "no-store", "no-cache") {
		return false
	}
	for _, prefix := range policy.Skip {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return false
		}
	}
	if len(policy.Paths) == 0 {
		return true
	}
	for _, prefix := range policy.Paths {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return true
		}
	}
	return false
}

func baseKey(r *http.Request) string {
	key := pagePrefix + r.Method + ":" + r.Host + r.URL.Path
	if r.URL.RawQuery != "" {
		key += "?" + r.URL.RawQuery
	}
	return key
}

func variantKey(r *http.Request, base string, names []string) string {
	if len(names) == 0 {
		return base
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+r.Header.Get(name))
	}
	return base + "|" + strings.Join(parts, "&")
}

func lookupPage(r *http.Request, c cache.Cache, base string) (pageEntry, bool) {
	names := []string{}
	if raw, found, err := c.Get(r.Context(), varyPrefix+base); err == nil && found {
		names = varyNames(string(raw))
	}

	raw, found, err := c.Get(r.Context(), variantKey(r, base, names))
	if err != nil || !found {
		return pageEntry{}, false
	}
	entry, err := decodePageEntry(raw)
	if err != nil {
		return pageEntry{}, false
	}
	return entry, true
}

func storePage(r *http.Request, c cache.Cache, base string, recorder *pageRecorder, policy settings.PageCache) {
	if !storable(recorder) {
		return
	}

	entry := recorder.entry()
	ttl := policy.TTL
	if fromResponse := maxAgeOf(entry.Header); fromResponse > 0 {
		ttl = fromResponse
	}

	body, err := entry.encode()
	if err != nil {
		return
	}
	if len(entry.Vary) > 0 {
		_ = c.Set(r.Context(), varyPrefix+base, []byte(strings.Join(entry.Vary, ", ")), ttl)
	}
	_ = c.Set(r.Context(), variantKey(r, base, entry.Vary), body, ttl)
}

func storable(recorder *pageRecorder) bool {
	if recorder.status != http.StatusOK || recorder.cookie {
		return false
	}
	if len(recorder.Header().Values("Set-Cookie")) > 0 {
		return false
	}
	header := recorder.Header()
	if hasDirective(header, "no-store", "private", "no-cache") {
		return false
	}
	if strings.TrimSpace(header.Get("Vary")) == "*" {
		return false
	}
	return true
}
