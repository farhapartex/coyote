# Architecture

[← Back to contents](README.md)

Useful if you are extending the framework, replacing a piece of it, or deciding whether it will bend
the way you need.

## Dependency direction

```
router ← middleware ┐
session, template   ├→ app → admin
auth, settings, db  ┘   ↑
model ──────────────────┴── contrib/cli → contrib/migrate
```

Three layers, and the arrows only point one way:

| Layer | Rule |
| --- | --- |
| `lib/` | imports nothing local — pure helpers, usable outside the framework |
| `core/` | the framework proper; never imports `contrib`, `admin` or `cmd` |
| `contrib/`, `admin/` | opt-in; may import `core` freely |

Two tests enforce this rather than trusting discipline: one asserts nothing under `lib/` imports a
local package, the other asserts nothing under `core/` reaches into `contrib`, `admin` or `cmd`.

There is exactly one accepted inversion — `core/app/serve.go` imports `contrib/cli` so `app.Run()`
can dispatch subcommands. It is listed explicitly in the test, which also fails if the exception ever
stops being needed. An undocumented shortcut breaks the build.

## Interfaces, not concrete types

What you can replace, and the contract you implement:

| Seam | Interface | Set it with |
| --- | --- | --- |
| Session storage | `session.Store` (+ `ManageableStore`) | `Sessions.Store` |
| User storage | `auth.Store` | `Auth.UserStore` |
| Permission storage | `auth.PermissionStore` | `Auth.PermissionStore` |
| Reset tokens | `auth.TokenStore` | registered when `Auth.ResetTokens` is on |
| File storage | `storage.Storage` | `Uploads.Storage` |
| Password rules | `auth.PasswordRule` | `Auth.PasswordRules` |
| Login limits | `auth.LoginLimiter` | supplied to the service |
| Pagination | `view.Paginator` | `Pagination.Paginator` |
| Validation rules | `form.Rule` | `form.Register` |
| Generic records | `model.Store` | supply to the admin |
| Admin resources | `admin.Resource` and its optional siblings | `portal.Manage` |
| Migration steps | `migrate.Op` | in a migration's `Up` |
| Config sources | `settings.Source` | `settings.Load` |
| Project template | files in `contrib/scaffold` | `coyote new` |
| Middleware | `func(http.Handler) http.Handler` | `Use`, `Group`, per route |

The admin portal depends on `model.Store` and never on `core/store`, which is why swapping the
persistence layer does not touch it.

## One responsibility per file

Each package is split by responsibility rather than by size — `core/settings` alone is
`settings.go`, `defaults.go`, `database.go`, `configure.go`, `normalize.go`, `accessors.go`,
`secret.go`, `env.go`, and five focused validators. No file in `core/`, `contrib/` or `admin/`
exceeds 200 lines.

## Standard library first

Routing, sessions, templates, authentication, middleware, and the admin portal are standard library
only. `net/http.ServeMux` does the dispatch, `html/template` does the rendering, `log/slog` does the
logging, `crypto/pbkdf2` hashes passwords. The dependencies that exist are GORM and its drivers for
persistence, and `golang.org/x/crypto` for ACME.

The practical consequence: anything written for `net/http` works here, and the concepts you already
know transfer instead of being re-taught.

## Where the seams are missing

Being explicit about what is not extensible yet:

- The user model is not swappable — the framework's own code still works in terms of `*auth.User`.
- Migrations are forward-only; there is no `down`, and they run against the default connection only,
  so a model routed to another alias is reported but not migrated.
- Rate limit and login-throttle buckets are per process, not shared.
- Full-text search is not built in; `model.Query` filters, but does not search.
- Relations cover belongs-to only — no has-many, no many-to-many.
- Soft delete is deliberately absent; it depends too much on the system to belong in the framework.

## Back to [contents](README.md)
