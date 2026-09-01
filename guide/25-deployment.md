# Deployment

[← Back to contents](README.md)

## Build

The SQLite driver is pure Go, so a static binary needs no C toolchain:

```
CGO_ENABLED=0 go build -o myapp .
```

Embed templates and static files (see [Templates](08-templates.md) and
[Static files](09-static-files.md)) and the binary is the whole deployment.

## Settings for a deployed environment

```go
settings.Configure(
	settings.Preset(settings.Env("APP_ENV", "development")),
	func(s *settings.Settings) {
		s.SecretKey    = settings.Env("SECRET_KEY", "")
		s.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", nil)
		s.Databases    = []settings.Database{{
			Engine:   settings.Postgres,
			Name:     settings.Env("DB_NAME", "app"),
			Host:     settings.Env("DB_HOST", "localhost"),
			User:     settings.Env("DB_USER", "app"),
			Password: settings.Env("DB_PASSWORD", ""),
		}}
	},
)
```

The `production` preset already turns `Debug` off, switches logging to JSON at `info`, requires
`AllowedHosts`, sets `Sessions.Secure`, caches templates, and applies server timeouts. Validation
refuses to start on an unsafe combination, so a bad config fails before it serves traffic.

**`Preset` is what applies that hardening, not the `Environment` value.** Setting
`s.Environment = settings.Production` by hand labels the environment and nothing more, so
validation asks for the pieces the preset would have set — `Sessions.Secure`,
`Server.WriteTimeout` — by name. That is deliberate: nothing rewrites a choice you made on purpose,
such as a longer write timeout for a streaming endpoint.

## Checklist

- [ ] `SecretKey` from the environment, at least 32 random characters, not in source control
- [ ] `AllowedHosts` listing the real host names — never `"*"`
- [ ] `Environment` set to `production` (or `staging`), ideally through `settings.Preset`
- [ ] TLS terminated, either [in-process](22-https.md) or by a proxy
- [ ] `Sessions.Secure = true` — refused otherwise, so this one cannot be forgotten
- [ ] `Server.ReadTimeout` and `Server.WriteTimeout` set — also refused otherwise
- [ ] Migrations applied as part of the release, before the new binary takes traffic
- [ ] A superadmin created once, then `createsuperadmin` no longer needed
- [ ] `Server.TLS.CacheDir` kept across deploys if you use Autocert
- [ ] Session backend chosen deliberately — `database` by default; `memory` is refused here
- [ ] [`Security.RateLimit`](19-rate-limiting.md) decided one way or the other

Rate limiting stays off unless you ask for it, and that is deliberate: the limiter sits in the same
chain as your static files, so a page with twenty assets costs twenty-one requests against the
allowance, and a shared office address arrives as one client. Pick numbers that fit your traffic, or
put the limit in the proxy in front. [Login throttling](14-authentication.md) is separate and is
already on.

## Sessions across restarts and instances

`memory` is per-process: a deploy signs everyone out, and two instances do not share sessions. Use
[`database`](13-sessions.md) when either matters, or `cookie` when you want no shared state at all
and can live without revocation.

## Shutdown

`Run` handles SIGINT and SIGTERM: it stops accepting connections, gives in-flight requests
`Server.ShutdownTimeout` (10s by default, 20s under the deployed presets) to finish, then closes the
database.

```
INFO shutting down signal=terminated
INFO stopped
```

## Logging

JSON at `info` in deployed environments. Every line carries the request id, which is also returned
as `X-Request-Id`, so a user-reported failure maps to its log line.

```go
s.Logging.Format = "json"
s.Logging.Logger = myLogger   // your own *slog.Logger, if you have one
```

**A failed query does not log its SQL unless `Debug` is on.** GORM hands back the statement with the
values already substituted, so logging it in production would put whatever the row held — an address,
an email, a token digest — into your log pipeline, where it outlives the request and travels
wherever logs travel. Deployed, the line carries the error, the row count and the duration; turn
`Debug` on to see the statement.

## Behind a proxy

Serve plain HTTP on a private address and let the proxy terminate TLS. Keep
`middleware.RequireHTTPS`, set `Sessions.Secure = true`, and set `Security.TrustedProxyCount` to the
number of proxies actually in front of the app — one for a single nginx or load balancer, two behind
a CDN as well. Nothing reads a forwarding header until you do. Turn on `Security.TrustRequestID`
**only** if that proxy sets `X-Request-Id` itself.

## Next

[Testing →](26-testing.md)
