# Settings

[← Back to contents](README.md)

Settings are declared in code, in one place, and validated before the server starts. There is no
implicit configuration: `app.New()` panics if `settings.Configure` has not run.

```
$ go run .
panic: coyote/settings: improperly configured: no settings have been configured

	Coyote needs an explicit settings file. Create settings.go next to your main package:
	...
```

## Declaring them

`Configure` takes functions that mutate a `Settings` value already filled with defaults, so you
write only what differs.

```go
settings.Configure(func(s *settings.Settings) {
	s.Debug = false
	s.SecretKey = settings.Env("SECRET_KEY", "")
	s.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", []string{"example.com"})
	s.Sessions.Secure = true
	s.Sessions.Lifetime = 24 * time.Hour
})
```

Call it once, from `init()` in `settings.go`. Calling it twice is an error.

At runtime the resolved values are available as `a.Settings`.

## Validation

Validation runs inside `Configure`, so mistakes surface at startup — all of them at once, not one
per restart:

```
panic: coyote/settings: improperly configured:
  - SecretKey is empty; set a random value of at least 32 characters
  - AllowedHosts is empty; with Debug disabled you must list the hosts this site serves
  - Sessions.SameSite must be "lax", "strict" or "none"
  - Admin.Prefix cannot be "/", it would take over every route
```

## Environments

Every project runs in one of three environments, and each carries a preset of sane defaults so you
are not hand-hardening production from scratch:

```go
settings.Configure(
	settings.Preset(settings.Env("APP_ENV", "development")),
	func(s *settings.Settings) {
		s.SecretKey = settings.Env("SECRET_KEY", "")
	},
)
```

A preset is just a mutator, so it composes with whatever follows — your overrides always win.

| | development | staging and production |
| --- | --- | --- |
| `Debug` | on | off |
| `Logging` | `debug`, text | `info`, json |
| `Sessions.Secure` | off | on |
| `AllowedHosts` | localhost if unset | must be set |
| Template caching | reload every request | cached |
| Server timeouts | none | read 15s, write 30s, shutdown 20s |

Names are forgiving — `dev`, `local`, `stage`, `prod`, `live` all resolve — but an unrecognised one
is a configuration error, not a silent fallback:

```
coyote/settings: improperly configured: Debug must be off when Environment is production
coyote/settings: improperly configured: Environment "banana" is not recognised; use "development", "staging" or "production"
```

`s.IsDevelopment()`, `s.IsStaging()`, `s.IsProduction()` and `s.IsDeployed()` are available to your
own code, and the active environment is logged at startup.

## Values from a file

Settings stay in code. What a file supplies is **values**, which `settings.go` reads through the
`Env` helpers — so secrets stay out of source control without creating a second place where
configuration is decided.

```go
func init() {
	settings.MustLoadDotEnv(".env")

	settings.Configure(settings.Preset(settings.Env("APP_ENV", "development")), func(s *settings.Settings) {
		s.SecretKey = settings.Env("SECRET_KEY", "")
		s.Server.Port = settings.EnvInt("PORT", 8000)
	})
}
```

```
# .env
APP_ENV=production
SECRET_KEY="a long random value"
ALLOWED_HOSTS=example.com, www.example.com
```

**A real environment variable always wins over the file**, so a container or CI can override
anything. With several files, the first to define a key wins. A missing `.env` is not an error; use
`settings.RequiredDotEnv(path)` when it must exist.

The parser handles comments, blank lines, an optional `export` prefix, single quotes as literals,
double quotes with `\n` escapes, inline comments, and empty values. A malformed line is reported
with its line number rather than skipped.

Other formats are one method away, which is why no YAML or TOML dependency is bundled:

```go
type Source interface {
	Name() string
	Values() (map[string]string, error)
}

settings.Load(myTOMLSource{path}, settings.DotEnv(".env"))
```

`settings.Map` is a built-in source, handy in tests.

### Env helpers

`settings.Env`, `EnvBool`, `EnvInt`, `EnvDuration`, `EnvList`. Each takes a key and a fallback. The
framework itself never reads the environment — only your `settings.go` does, where you can see it.

## Every setting and its default

| Setting | Default | Notes |
| --- | --- | --- |
| `Debug` | `false` | Template reload, verbose render errors, relaxed host checks |
| `Environment` | `development` | `development`, `staging` or `production` |
| `BaseDir` | working directory | Root that relative paths resolve against |
| `SecretKey` | none | Required. Under `Debug` an ephemeral key is generated with a warning |
| `AllowedHosts` | none | Required unless `Debug`. `"*"` allows any, `".example.com"` matches subdomains |
| `Databases` | one SQLite entry at `<BaseDir>/coyote.db` | First entry is the default connection |
| `Caches` | one in-memory entry | First entry is the default cache; see [Caching](33-caching.md) |
| `I18N` | one locale, `en` | Locales, catalogs, detection; see [Internationalisation](34-internationalisation.md) |
| `TimeZone` | `UTC` | Validated at startup; needs `time/tzdata` on a scratch image |
| `PageCache` | off | Whole-page caching; see [Caching](33-caching.md) |
| `Server.Host` | `127.0.0.1` | |
| `Server.Port` | `8000` | |
| `Server.MaxBodyBytes` | `32 MB` | Cap on any request body; 0 lifts it |
| `Server.ReadTimeout` | `15s` | Whole-request read deadline |
| `Server.WriteTimeout` | none, `30s` when deployed | Left unset so streaming works; required when deployed |
| `Server.IdleTimeout` | `2m` | |
| `Server.ReadHeaderTimeout` | `10s` | |
| `Server.ShutdownTimeout` | `10s` | Grace period on SIGINT/SIGTERM |
| `Server.TLS.CertFile` / `.KeyFile` | none | Both set means serve HTTPS |
| `Server.TLS.MinVersion` | TLS 1.2 | |
| `Server.TLS.HSTS` | none | Adds `Strict-Transport-Security`; requires TLS |
| `Server.TLS.Config` | none | A `*tls.Config` used as-is |
| `Server.TLS.Autocert` | `false` | Obtain and renew certificates from Let's Encrypt |
| `Server.TLS.AcceptTOS` | `false` | Required with `Autocert` |
| `Server.TLS.CacheDir` | `certs` | Where issued certificates are stored |
| `Server.TLS.Staging` | `false` | Use the ACME staging directory while testing |
| `Server.Configure` | none | `func(*http.Server)` hook called before listening |
| `Security.TrustedProxyCount` | `0` | Proxies in front; how far into `X-Forwarded-For` to trust |
| `Security.CSRF` | `true` | Guard every unsafe request — [CSRF](18-csrf.md) |
| `Security.CSRFExempt` | none | Path prefixes the guard skips |
| `Security.FrameOptions` | `DENY` | `DENY`, `SAMEORIGIN`, or empty to omit |
| `Security.PermissionsPolicy` | camera, mic, geo denied | Sent as `Permissions-Policy` |
| `Security.CSP` | none | Content Security Policy; off until set |
| `Security.CSPReportOnly` | `false` | Report violations instead of blocking |
| `Security.CORS` | off | See [CORS](20-cors.md) |
| `Security.Compress` | `false` | gzip responses |
| `Security.CompressLevel` | `0` | 1–9, or 0 for the default |
| `Security.RateLimit` | off | See [Rate limiting](19-rate-limiting.md) |
| `Security.TrustRequestID` | `false` | Accept an inbound `X-Request-Id` |
| `Sessions.Backend` | `database` | `database`, `memory` or `cookie`; memory is refused when deployed |
| `Sessions.CookieName` | `coyote_session` | |
| `Sessions.Lifetime` | `12h` | |
| `Sessions.Rolling` | `false` | Extend the deadline on every request |
| `Sessions.Secure` | `false` | Set true behind HTTPS |
| `Sessions.HTTPOnly` | `true` | |
| `Sessions.SameSite` | `lax` | `lax`, `strict` or `none` (requires `Secure`) |
| `Sessions.Path` | `/` | |
| `Sessions.Domain` | none | |
| `Sessions.CleanupInterval` | `5m` | Expired-session sweep |
| `Sessions.Store` | none | Supply your own `session.Store` |
| `Auth.LoginURL` | `/admin/login` | Where guards send anonymous visitors |
| `Auth.PasswordMinLength` | `8` | |
| `Auth.PasswordRules` | four defaults | Replace to change the policy; see [Authentication](14-authentication.md) |
| `Auth.PBKDF2Iterations` | `600000` | Lower it in tests to keep them fast |
| `Auth.Throttle` | on, 5 per 15 min | Login attempt limits; see [Authentication](14-authentication.md) |
| `Auth.Permissions` | `true` | Register the permission and role tables; see [Permissions](28-permissions.md) |
| `Auth.AllowPasswordChange` | `true` | Off hides the form and makes the route 404 |
| `Auth.ResetTokens` | `false` | Register the reset-token table |
| `Auth.ResetTokenLifetime` | `1h` | How long a reset token stays valid |
| `Auth.PermissionStore` | database-backed | Any `auth.PermissionStore` |
| `Auth.UserStore` | database-backed | Any `auth.Store`; falls back to memory with no database |
| `Templates.FS` | none | An `fs.FS`, usually from `go:embed` |
| `Templates.Dir` | none | A directory path instead of an `fs.FS`; mutually exclusive with `FS` |
| `Templates.Layout` | `layouts/base.html` | |
| `Templates.Shared` | `layouts/*.html`, `partials/*.html` | Parsed into every page |
| `Templates.Funcs` | none | Extra template functions |
| `Pagination.PerPage` | `10` | 0 turns paging off; see [Pagination](32-pagination.md) |
| `Pagination.Paginator` | built-in | Any `view.Paginator` |
| `Uploads.Enabled` | `false` | Turns the upload service on |
| `Uploads.Dir` | `media` | Where files live, under `BaseDir` |
| `Uploads.Path` | none | Default path inside the media directory; a field tag overrides it |
| `Uploads.MaxSize` | `10 MiB` | Enforced before the body is read |
| `Uploads.Allowed` | images and PDF | Content types, matched against the sniffed type |
| `Uploads.MaxPixels` | `50,000,000` | Decompression-bomb guard; an image that will not decode is refused |
| `Uploads.Serve` | `false` | Serve uploads over HTTP at `Uploads.URL` |
| `Uploads.URL` | `/media/` | |
| `Uploads.Private` | `false` | Require a signed URL; needs `SecretKey` |
| `Uploads.SignedURLTTL` | `15m` | How long a signed media link stays valid |
| `Uploads.StageTTL` | `24h` | How long an uncommitted upload survives |
| `Uploads.TrashTTL` | `0` | 0 deletes immediately; above zero keeps a recovery window |
| `Uploads.Storage` | filesystem | Any `storage.Storage` |
| `Static.URL` | `/static/` | |
| `Static.FS` / `Static.Dir` | none | Static files are served only when one is set |
| `Migrations.Dir` | `migrations` | Where generated migrations are written |
| `Admin.Prefix` | `/admin` | |
| `Admin.SiteName` | `Coyote administration` | |
| `Admin.Tagline` | none | |
| `Logging.Level` | `info` | `debug`, `info`, `warn`, `error` |
| `Logging.Format` | `text` | `text` or `json` |
| `Logging.Logger` | none | Supply your own `*slog.Logger` |
| `Email.Backend` | none | `smtp`, `console`, `file`, `memory`; empty sends nothing |
| `Email.Host` / `Email.Port` | none / by TLS mode | SMTP only; 587 starttls, 465 tls, 25 none |
| `Email.Username` / `Email.Password` | none | SMTP only; refused with `Email.TLS` `none` |
| `Email.TLS` | `starttls` | `none`, `starttls` or `tls` |
| `Email.From` | none | The SMTP backend's default sender; other backends need `From` on the message, or `mail.WithDefaultFrom` |
| `Email.Dir` | `mail` | File backend only; relative to `BaseDir` |
| `Email.Timeout` | `10s` | Per send |
| `Email.LocalName` | `localhost` | The name given in `EHLO` |
| `Email.Sender` | none | Any `mail.Sender`; cannot be combined with `Email.Backend` |

## Next

- [Internationalisation →](34-internationalisation.md) — the `I18N` group in detail
- [Caching →](33-caching.md) — the `Caches` list in detail
- [Email →](35-email.md) — the `Email` group in detail
- [Databases →](11-database.md) — the `Databases` list in detail
- [Routing →](05-routing.md)
