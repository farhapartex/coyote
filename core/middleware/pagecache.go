package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/settings"
)

const (
	CacheStatusHeader = "X-Cache"
	CacheHit          = "HIT"
	CacheMiss         = "MISS"

	pagePrefix = "page:"
	varyPrefix = "page:vary:"
)

func PageCache(c cache.Cache, policy settings.PageCache, sessionCookie string) Middleware {
	if c == nil || policy.TTL <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cacheableRequest(r, policy, sessionCookie) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Cookie")

			base := baseKey(r, policy.Vary)
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

func cacheableRequest(r *http.Request, policy settings.PageCache, sessionCookie string) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	if carriesSession(r, sessionCookie) {
		return false
	}
	if hasDirective(r.Header, "no-store", "no-cache") {
		return false
	}
	if matchesAnyPrefix(r.URL.Path, policy.Skip) {
		return false
	}
	return len(policy.Paths) == 0 || matchesAnyPrefix(r.URL.Path, policy.Paths)
}

func carriesSession(r *http.Request, cookieName string) bool {
	if cookieName == "" {
		return false
	}
	_, err := r.Cookie(cookieName)
	return err == nil
}

func baseKey(r *http.Request, vary []string) string {
	key := pagePrefix + r.Method + ":" + r.Host + r.URL.Path
	if query := keyedQuery(r, vary); query != "" {
		key += "?" + query
	}
	if locale := i18n.From(r.Context()); locale != nil {
		key += "#" + locale.Tag()
	}
	return key
}

func keyedQuery(r *http.Request, vary []string) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	if len(vary) == 0 {
		return r.URL.RawQuery
	}
	asked := r.URL.Query()
	kept := url.Values{}
	for _, name := range vary {
		if values, found := asked[name]; found {
			kept[name] = values
		}
	}
	return kept.Encode()
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
		names = keyedVaryNames([]string{string(raw)})
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
