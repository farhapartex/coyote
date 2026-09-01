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

By IP, by default, and that IP is the connection's peer address. `X-Forwarded-For` is **ignored until
you say how many proxies sit in front of the app**:

```go
s.Security.TrustedProxyCount = 1   // one nginx, one ALB, one anything
```

Count the hops, do not name them. A conforming proxy *appends* the address it received from, so with
one proxy in front the last entry is the one your proxy wrote and everything to its left came from
the client. Reading the header from the right by that many hops is the only way to tell the two
apart:

```
X-Forwarded-For: 9.9.9.9, 203.0.113.7
                 ^ the client typed this
                             ^ your proxy appended this
TrustedProxyCount = 1  ->  the client is 203.0.113.7
```

Get the count wrong and you trust one hop too many or too few, so it is worth checking against a
real request. If the chain is shorter than the count — someone stripped the header, or the count is
too high — the peer address is used instead, which is always safe and never forgeable.

`X-Real-Ip` is not read. It carries a single value with no hop structure, so nothing distinguishes
one your proxy set from one a client invented.

The same setting feeds [login throttling](14-authentication.md) and
[`RequireHTTPS`](22-https.md) — request trust is one decision, made once.

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
