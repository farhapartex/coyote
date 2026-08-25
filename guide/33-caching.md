# Caching

[← Back to contents](README.md)

An in-memory cache exists from the first line you write. Redis is one setting away, and nothing about
your code changes when you switch.

```go
totals, err := cache.Remember(r.Context(), a.Cache(), "dashboard:totals", time.Minute,
	func(ctx context.Context) (Totals, error) {
		return computeTotals(ctx)
	})
```

## Redis is not installed with the framework

It cannot be: `go get` distributes Go packages, not daemons. And a framework whose front page promises
that `go get` is the whole install cannot then require a service to be running before `coyote start`
works.

So the default cache is in-process, and Redis is opt-in. The client is written against RESP with the
standard library, so turning Redis on adds **no dependency** to your `go.mod`.

## The three backends

| | shared between processes | survives a restart | needs anything installed |
| --- | --- | --- | --- |
| `memory` | no | no | no |
| `file` | no (one machine) | yes | no |
| `redis` | yes | yes | Redis |

```go
s.Caches = []settings.Cache{{
	Backend:  settings.CacheInRedis,
	Address:  settings.Env("REDIS_ADDR", "127.0.0.1:6379"),
	Password: settings.Env("REDIS_PASSWORD", ""),
	TTL:      10 * time.Minute,
}}
```

`Caches` is a list and **the first entry is the default cache**, exactly like `Databases`. Blank aliases
become `default`, then `cache1`, `cache2`.

### A configured cache that is missing stops the server

This is deliberate, and it differs from the database:

```
$ coyote start
coyote: coyote/cache: the "default" cache is unreachable
    backend redis at 127.0.0.1:6379
    coyote/cache/redis: dialling 127.0.0.1:6379: dial tcp: connect: connection refused

  start Redis, or set Caches[0].Backend to "memory" or "file" in settings.go
```

Configuring Redis is a statement that Redis exists. If it does not, that is a misconfiguration, and you
should find out from a failed deploy rather than from a silent performance cliff. Every configured cache
is built and pinged before the listener opens; a file cache whose directory cannot be written fails the
same way.

**Once the server is up, the rule inverts.** A cache that dies while running degrades instead of
crashing the process: reads miss, writes are dropped, the failure is logged once, and `Stats().Errors`
counts the rest. Strict at the boundary, forgiving afterwards.

`coyote migrate` and the other commands do not run this check — they never touch the cache.

## Using it

`a.Cache()` never returns an error and is never nil, so a read you do not depend on needs no error
handling:

```go
c := a.Cache()

c.Set(r.Context(), "greeting", []byte("hello"), 0)
raw, found, err := c.Get(r.Context(), "greeting")
c.Delete(r.Context(), "greeting")
c.Has(r.Context(), "greeting")
c.Clear(r.Context())
```

Reach a named cache by alias:

```go
pages, err := a.CacheByAlias("pages")
```

That errors only for an alias you never configured, which is a bug in your code rather than a fact about
the environment.

### Remember

The helper you will actually use. One call replaces get-miss-build-set:

```go
products, err := cache.Remember(r.Context(), a.Cache(), "catalogue:page:1", time.Minute,
	func(ctx context.Context) ([]Product, error) {
		return loadProducts(ctx)
	})
```

- Values are encoded with **JSON by default**, into and out of a named type, so an `int` field comes
  back an `int`. (Sessions use gob because they decode into `any`, where JSON turns every number into a
  `float64`. A cache decodes into `T`, so JSON is safe here.) Set `Codec` on the cache to change it.
- **A failed build is never cached** and the error reaches you. Caching a failure is how a momentary
  database blip becomes a five-minute outage.
- **Concurrent misses on one key build once.** Ten requests arriving together run the function once and
  the other nine wait for its result. That collapsing is per process, like the rate-limit and
  login-throttle counters — behind three instances you get at most three builds, not one.
- If the cache is down, `Remember` still returns the built value. Your handler has one error path.

`cache.GetOrSet` is the same thing for raw bytes; `cache.GetJSON` and `cache.SetJSON` are the typed
get and set on their own.

## Time to live

Resolution runs **per call → the cache's `TTL` → 5 minutes**, worded the same way as
[pagination](32-pagination.md) so the two behave alike.

| `ttl` argument | Meaning |
| --- | --- |
| a positive duration | that duration |
| `0` | the cache's own `TTL` |
| `cache.Forever` | no expiry |

## Key prefix and version

Every key is stored as `<Prefix>:<Version>:<your key>`, with `Prefix` defaulting to `coyote`.

```go
s.Caches = []settings.Cache{{
	Backend: settings.CacheInRedis,
	Version: settings.Env("DEPLOY_ID", "1"),
}}
```

Bumping `Version` makes the whole namespace unreachable in one edit — useful when a deploy changes the
shape of what you cached.

One honest caveat: on Redis those old keys become **unreachable, not deleted**. They sit there until
their own TTL expires. If you want the memory back at once, call `Clear`.

## Clear only ever clears your own namespace

`Clear` deletes the keys under this cache's prefix and version, and nothing else. On Redis it is a
`SCAN` plus batched `DEL` — **never `FLUSHDB`**, because your Coyote app may be one of several tenants
in that database, and wiping a neighbour's keys because someone clicked "clear cache" is indefensible.

Two aliases can therefore share one Redis database safely, as long as their prefixes differ.

## Watching it work

```go
for _, stats := range a.CacheStats() {
	log.Printf("%s %s: %.0f%% of %d lookups, %d entries",
		stats.Alias, stats.Backend, stats.HitRate()*100, stats.Lookups(), stats.Entries)
}
```

`Stats` carries `Hits`, `Misses`, `Sets`, `Deletes`, `Errors`, `Evictions` and `Entries`, plus
`HitRate()` and `Lookups()`. There is no admin page for this on purpose — cache behaviour is a
developer's concern, and the portal is for people administering data. Log it, or put it on a page of
your own.

Two things the numbers do not tell you:

- They are **per process**, like the rate-limit buckets. Behind three instances you are seeing a third
  of the traffic.
- `Entries` is `-1` for Redis. `DBSIZE` counts the whole database including keys that are not yours, and
  counting your namespace means a full `SCAN` per call. A number that means something else is worse than
  no number.

## Caching a template fragment

```html
{{fragment "navbar.html" 300 (navkey .) .}}
```

Name, lifetime in seconds, **key part**, then the data. The named template must be one of your
`Templates.Shared` files — a fragment is a partial.

**The key is what you pass, not what you render.** Hashing arbitrary template data would be expensive
and unreliable, so the framework asks you to say what makes this render distinct. Forget the user, and
one visitor's sidebar is served to everybody. That is the whole risk of fragment caching, and it is why
the key is explicit rather than guessed.

Two safeguards:

- A fragment whose output contains the request's **CSRF token or CSP nonce is never stored**. It renders
  normally and logs why it was not cached. A shared CSRF token is the worst bug this feature could
  cause, so it is unreachable by accident rather than merely discouraged.
- With no cache configured, `fragment` renders straight through, so the template works either way.

## Caching whole pages

Off unless you ask for it:

```go
s.PageCache = settings.PageCache{
	Enabled: true,
	TTL:     10 * time.Minute,
	Paths:   []string{"/docs"},
	Skip:    []string{"/docs/private"},
}
```

Or apply it to one group yourself:

```go
docs := a.Group("/docs", middleware.PageCache(a.Cache(), settings.PageCache{TTL: time.Hour}))
```

A response is stored only when **all** of these hold: the method is GET or HEAD, the status is 200, the
response set no `Set-Cookie`, the request carried no `Authorization` header, and `Cache-Control` says
neither `no-store` nor `private`. Responses carry `X-Cache: HIT` or `MISS`.

The `Set-Cookie` rule is the one doing the real work. Sessions are written
[lazily](13-sessions.md), and a session that stays empty is never written at all — so anonymous traffic
emits no cookie and is cacheable, while anything that touched a session emits one and is not.
Personalised pages exclude themselves without you configuring anything.

**The corollary catches people out:** `{{.CSRFToken}}` mints a token, which writes the session, which
sets a cookie. So a page carrying a form is never cached — correct, but if the form lives in a partial
that every page shares, such as a search box or a language picker in the nav, you have turned page
caching off for the whole site. Keep CSRF-protected forms out of shared partials, or accept that those
pages are uncacheable.

`Vary` is honoured: the response's `Vary` header names are recorded, and the body is keyed by those
request headers, so two languages get two entries. `Vary: *` is never cached. A `max-age` or `s-maxage`
on the response overrides the policy TTL.

The middleware sits **inside** `Compress`, so one uncompressed copy is stored and gzip runs per
response. Do not move it outside, or you will serve gzip to clients that did not ask for it.

## Caching generic-store reads

Opt in per table, and only per table:

```go
records, _ := a.Store()
cached := store.Cached(records, a.Cache(), store.CacheOptions{
	Tables: []string{"products"},
	TTL:    2 * time.Minute,
})
```

`Find`, `List`, `Count`, `First` and `Exists` are cached; writes through the same store invalidate the
whole table at once via a generation counter, so an `Insert` costs one increment rather than a scan.
Records are encoded with **gob**, not JSON, because a `model.Record` is a `map[string]any` — the one
place where JSON would turn your `int64` into a `float64` and your `time.Time` into a string.

Four rules worth knowing before you switch it on:

- **A read inside a transaction is never cached.** `WithTx` hands back the uncached store, because a
  transaction must see its own uncommitted writes and must not publish them to anyone else.
- Lists with no `Limit` are not cached.
- The admin portal is not cached, and should not be — an operator looking at a record needs the truth.
- **A write from anywhere else does not invalidate anything.** `coyote dbshell`, another service, a
  migration: none of them know about your cache. Keep the TTL short, and treat this as an optimisation
  for read-heavy tables rather than a default.

## Your own backend

```go
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Has(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
}
```

```go
s.Caches = []settings.Cache{{Backend: settings.CacheInMemory, Store: myBackend{}}}
```

The interface is deliberately **byte-oriented**. An `any` cache would let the memory backend hand out a
live pointer while Redis hands out a copy, so code that works in development would mutate shared state
in production. Bytes make every backend behave identically, and the typed helpers sit on top.

What a backend receives, since none of it is visible in the signatures:

- Keys arrive **already namespaced**. Do not add a prefix of your own.
- `ttl <= 0` means **no expiry**. The default-TTL and `Forever` resolution happens before your backend
  is called.
- A miss is `(nil, false, nil)`. Reserve the error for a backend that actually failed.
- `Get` and `Set` should **copy** the bytes they are given and return, so a caller mutating a slice
  cannot corrupt the cache. Redis and the file backend get this for free; an in-memory one must do it.

Implement any of these and the framework will use them; skip them and it falls back:

| Interface | Gains |
| --- | --- |
| `Namespacer` | namespace-scoped `Clear` instead of clearing everything |
| `Counter` | one-increment invalidation in `store.Cached` |
| `Multi` | `GetMulti` |
| `Pinger` | the startup reachability check |
| `Measured` | `Entries` in `Stats` |
| `Evicter` | `Evictions` in `Stats` |

Cluster and Sentinel are not supported by the built-in Redis client. If you need either, wrap
`github.com/redis/go-redis/v9` in the interface above and set it as `Store` — that is precisely why the
seam exists.

## Every setting

| Setting | Default | Notes |
| --- | --- | --- |
| `Caches` | one `memory` entry | First entry is the default cache |
| `Caches[].Alias` | `default`, then `cache1`… | Must be unique |
| `Caches[].Backend` | `memory` | `memory`, `redis` or `file` |
| `Caches[].TTL` | `5m` | Default lifetime; a per-call value wins |
| `Caches[].Prefix` | `coyote` | First half of the key namespace |
| `Caches[].Version` | none | Second half; bump to invalidate a deploy |
| `Caches[].Address` | `127.0.0.1:6379` | Redis only |
| `Caches[].Username` / `.Password` | none | Redis only |
| `Caches[].Database` | `0` | Redis database index |
| `Caches[].TLS` | none | A `*tls.Config` for Redis |
| `Caches[].Dir` | `cache` | File backend, resolved against `BaseDir` |
| `Caches[].MaxEntries` | `10000` | Memory backend; LRU eviction above it |
| `Caches[].MaxBytes` | none | Memory backend, additional bound |
| `Caches[].CleanupInterval` | `5m` | Expiry sweep for memory and file |
| `Caches[].PoolSize` | `8` | Redis connections |
| `Caches[].DialTimeout` | `2s` | Also the startup check's patience |
| `Caches[].ReadTimeout` / `.WriteTimeout` | `3s` | Redis only |
| `Caches[].Store` | none | Your own `cache.Cache` |
| `PageCache.Enabled` | `false` | |
| `PageCache.TTL` | none | Required when enabled |
| `PageCache.Alias` | the default cache | Which cache holds pages |
| `PageCache.Paths` | all paths | Only cache these prefixes |
| `PageCache.Skip` | none | Never cache these prefixes |

## Next

[Email →](35-email.md)
