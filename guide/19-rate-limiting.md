# Rate limiting

[← Back to contents](README.md)

Off until you set a policy.

```go
s.Security.RateLimit = settings.RateLimit{
	Requests: 60,
	Window:   time.Minute,
	Burst:    100,
}
```

A token bucket per client: `Requests` per `Window` is the sustained rate, `Burst` the short-term
ceiling. Over the limit returns **429** with `Retry-After`; every response carries `RateLimit-Limit`
and `RateLimit-Remaining`.

## Identifying clients

By IP, by default. `X-Forwarded-For` is **ignored unless `TrustProxy` is set**, because without a
proxy in front, anyone can send that header and mint themselves a fresh budget on every request.

```go
s.Security.RateLimit.TrustProxy = true   // only with a proxy you control
```

For anything other than an IP — an API key, a tenant, an account — supply your own key function:

```go
a.Use(middleware.RateLimitBy(policy, func(r *http.Request) string {
	return r.Header.Get("X-Api-Key")
}))
```

`middleware.ClientIP(trustProxy)` is the built-in key function, if you want to compose with it.

## Scope

Applied globally through settings. To limit one area only, skip the setting and apply the
middleware to a group:

```go
api := a.Group("/api", middleware.RateLimit(settings.RateLimit{Requests: 10, Window: time.Minute}))
```

## What it is not

Buckets live in memory and stale ones are swept, so the map does not grow with every unique visitor.
That also means **each process has its own budget**: behind several instances the effective limit is
per instance until a shared backend exists. It is a courtesy limit and a brute-force speed bump, not
a defence against a distributed flood — that belongs upstream.

## Next

[CORS →](20-cors.md)
