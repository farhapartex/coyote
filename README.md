<p align="center">
  <img src="./assets/coyote.png" alt="Coyote" width="220">
</p>

<h1 align="center">coyote</h1>

A session-first web framework for Go, shaped like Django but sized for the standard library.
Routing, sessions, auth, templates, and the admin portal are pure standard library; persistence
uses GORM with a cgo-free SQLite driver.

Configuration is explicit: every project declares a `settings.go`, and the app refuses to start
without one. It ships with settings, sessions, authentication, HTML template rendering, and a
working admin portal.

Authentication state lives in the session; everything else is persisted, with a SQLite database
configured out of the box. Users are stored in the database, so an account created with
`createsuperadmin` in one process is there for the server in another. Sessions are still held in
memory, which means a restart signs everyone out.

## Install

```
go get github.com/farhapartex/coyote
```

Requires Go 1.24 or newer (`crypto/pbkdf2`, tool directives).

## Quick start

Every Coyote project needs two files. First `settings.go`, which is mandatory — the app refuses
to start without it:

```go
package main

import (
	"embed"
	"io/fs"

	"github.com/farhapartex/coyote/core/settings"
)

//go:embed templates
var templateFS embed.FS

func init() {
	templates, _ := fs.Sub(templateFS, "templates")

	settings.Configure(func(s *settings.Settings) {
		s.Debug = true
		s.SecretKey = settings.Env("SECRET_KEY", "development-only-key-change-me-please")
		s.AllowedHosts = []string{"127.0.0.1", "localhost"}
		s.Server.Port = 8000
		s.Templates.FS = templates
		s.Admin.SiteName = "My site admin"
	})
}
```

Then `main.go`, which reads it:

```go
package main

import (
	"log"
	"net/http"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
)

func main() {
	a := app.New()

	a.Auth.CreateSuperadmin("admin", "admin@example.com", "coyote123")
	admin.Mount(a)

	a.Get("/", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})
	})

	log.Fatal(a.Run())
}
```

Then open `http://localhost:8000/` and `http://localhost:8000/admin/`.

## Running the example

```
cd example
go run .
```

Serves on `127.0.0.1:8000`. `example/settings.go` reads `PORT`, `HOST`, `DEBUG`, `SECRET_KEY`,
`ALLOWED_HOSTS`, and `DB_NAME` from the environment, so `PORT=9000 go run .` works. It declares a
SQLite database at `example/coyote.db`; the file is not created yet because nothing connects. The
resolved config is visible at `/about` and under the admin portal's Server info section. Two users
are seeded:

| Username | Password    | Role |
| -------- | ----------- | ---- |
| `admin`  | `coyote123` | superadmin |
| `editor` | `coyote123` | plain user (cannot reach the admin portal) |

## Commands

Coyote ships one management command, registered as a Go tool so it is versioned with your project:

```
go get -tool github.com/farhapartex/coyote/cmd/coyote
```

Run it from the directory holding your `main.go` and `settings.go`.

| Command | What it does |
| --- | --- |
| `go tool coyote start` | Build and run the project, reporting migration and user state first |
| `go tool coyote makemigrations` | Diff your models against the snapshot and write a migration file |
| `go tool coyote migrate` | Apply migrations that have not been applied yet |
| `go tool coyote sqlmigrate` | Print the SQL a pending migration would run, without applying it |
| `go tool coyote createsuperadmin` | Create a superadmin who can sign in to the admin portal |
| `go tool coyote version` | Print the coyote version |
| `go tool coyote help` | List the commands |

| Flag | Command | Purpose |
| --- | --- | --- |
| `--port=N` | `start` | Listen on N instead of the port in `settings.go` |
| `--host=H` | `start` | Bind to H instead of the host in `settings.go` |
| `--name=NAME` | `makemigrations` | Name the migration instead of guessing one |
| `--username=U` | `createsuperadmin` | Skip the username prompt |
| `--email=E` | `createsuperadmin` | Skip the email prompt |
| `--password=P` | `createsuperadmin` | Skip the password prompt |

`createsuperadmin` also reads `COYOTE_SUPERADMIN_USERNAME`, `_EMAIL` and `_PASSWORD`, which is the
safer route in scripts since the prompt echoes what you type.

A new project starts like this:

```
go tool coyote makemigrations --name=initial
go tool coyote migrate
go tool coyote createsuperadmin
go tool coyote start
```

`go coyote start` is not possible — the `go` command cannot be extended with new subcommands, and
`go tool` is the closest supported form. The tool builds and runs the main package in the current
directory, so your own `settings.go` applies; it passes the subcommand through `COYOTE_COMMAND` and
any overrides through `COYOTE_PORT` / `COYOTE_HOST`. Your `main.go` needs nothing beyond
`app.New()` and `Run()` — `Run` dispatches to whichever command was asked for.

Note that `migrate` currently refuses unless the server is already listening on the configured
address.

## Layout

Coyote follows MVT. `core/` holds the framework; `contrib/` holds tooling built on top of it;
`admin/`, `cmd/`, `tests/`, and `example/` sit beside them.

```
lib/
  text/         casing, slugs, folding, truncation — no framework dependencies
  dotenv/       the .env parser, io.Reader in, map out
  id/           UUID generation
core/
  app/          the application object: wiring, lifecycle, render entry point
  router/       URL dispatch — groups, method helpers, static, route table
  view/         view helpers — template data, redirects, flash messages      (V)
  template/     the html/template engine with layouts and partials          (T)
  model/        entity metadata: schema introspection, registry, Store port  (M)
  store/        the GORM adapters behind model.Store and auth.Store          (M)
  auth/         the User entity, passwords, login guards                     (M)
  db/           GORM connection, pool tuning, pragmas, slog bridge
  session/      Session, Store interface, MemoryStore, cookie manager
  middleware/   logging, recovery, allowed hosts, secure headers, CSRF
  settings/     the settings type, defaults, validation, env helpers
contrib/
  admin/        the admin portal — optional, mounted by your app
  cli/          the management commands and their registry
  migrate/      migration operations, diffing, generation, the ledger
cmd/coyote/     the `go tool coyote` front end
tests/          the whole test suite, one package, black box
example/        a small site using the framework
```

`core` is what the framework needs to exist; `contrib` is what you opt into. Nothing in `core`
imports `contrib/admin`, which is why the admin portal lives there — you either call
`admin.Mount(app)` or you never compile it in.

Each package is split one responsibility per file — `core/settings` alone is `settings.go`,
`defaults.go`, `database.go`, `configure.go`, `normalize.go`, `accessors.go`, `secret.go`, `env.go`,
and five focused validators. No file in `core/`, `contrib/`, or `admin/` exceeds 200 lines.

MVT maps on as follows. **Model** is `core/model` plus the entities themselves (`auth.User` today).
**View** is your handlers plus `core/view`. **Template** is `core/template`. `core/router` plays
the part of Django's `urls.py`.

### Dependency direction

```
router ← middleware ┐
session, template   ├→ app → admin
auth, settings, db  ┘   ↑
model ──────────────────┴── contrib/cli → contrib/migrate
```

Two tests enforce this rather than trusting discipline: one asserts nothing under `lib/` imports a
local package, the other asserts nothing under `core/` reaches into `contrib`, `admin` or `cmd`.

There is exactly one accepted inversion — `core/app/serve.go` imports `contrib/cli` so `app.Run()`
can dispatch subcommands. It is listed explicitly in the test, which also fails if the exception
ever stops being needed.

### Commands

Coyote ships a management command, registered as a Go tool so it is versioned with your project:

```
go get -tool github.com/farhapartex/coyote/cmd/coyote

go tool coyote start                 # serve on the port from settings.go
go tool coyote start --port=5000     # override it for this run
go tool coyote migrate               # apply pending schema changes
```

See [Commands](#commands) for the full list. `migrate` refuses to do anything unless the server is
already listening on the configured address:

```
$ go tool coyote migrate
coyote/cli: server is not running at 127.0.0.1:8081; start it first with: go tool coyote start
```

With the server up it plans, then applies:

```
$ go tool coyote migrate
server    running on 127.0.0.1:8081
database  sqlite /path/to/example/coyote.db
models    1 registered

  create table users ... ok

applied 1 change(s)
```

Running it again reports `schema is up to date, nothing to apply`.

### Migrations

Schema changes are versioned. You edit a model, generate a migration, review it, commit it, and
apply it — and the framework records what ran so nothing runs twice.

```
edit model  →  makemigrations  →  review + commit  →  migrate
                    ↑ snapshot.json                     ↓ coyote_migrations
```

```
$ go tool coyote makemigrations --name=add_product_stock
$ go tool coyote sqlmigrate          # print the SQL without applying it
$ go tool coyote migrate
```

`makemigrations` diffs your structs against `migrations/snapshot.json`, not against the live
database, so it works with nothing connected — on a fresh clone or in CI. It writes a numbered Go
file and updates the snapshot; commit both together.

```go
package migrations

// Generated by: go tool coyote makemigrations
// Edit freely before applying — this file is the source of truth once committed.

import (
	"github.com/farhapartex/coyote/core/model"

	"github.com/farhapartex/coyote/contrib/migrate"
)

func init() {
	migrate.Register(migrate.Migration{
		ID: "0002_add_product_stock",
		Up: []migrate.Op{
			// add column products.stock
			migrate.AddColumn{Table: "products", Column: {Name: "stock", Kind: model.KindInt}},
		},
	})
}
```

Migration files are Go, not `.sql`, so one file works on every engine — operations render to
SQLite, Postgres, or MySQL at apply time. Blank import the package once from your main package so
the `init` functions run:

```go
import _ "your/module/migrations"
```

**Operations:** `CreateTable`, `DropTable`, `AddColumn`, `DropColumn`, `RenameColumn`,
`CreateIndex`, `DropIndex`, plus two escape hatches:

```go
migrate.RunSQL{SQLite: "...", Postgres: "...", Note: "why"}

migrate.RunGo{Note: "backfill slugs", Func: func(ctx context.Context, tx *gorm.DB) error {
	return tx.Table("products").Where("slug = ''").
		Update("slug", gorm.Expr("lower(name)")).Error
}}
```

`RunGo` is the reason migrations are Go files: a data migration can call your own code, and the
ledger guarantees it runs exactly once.

**Guarantees.** Each migration applies inside a transaction — a failure rolls back the schema
change *and* the ledger row together, so a half-applied migration cannot be recorded. The ledger
stores a checksum of every migration, so editing one that already ran is caught rather than
silently skipped:

```
coyote/migrate: a migration changed after it was applied: 0002_add_stock
  was applied as 6f1c… but is now 91ab…
```

**What is deliberately manual.** The diff generates drops and warns about them; it never guesses a
rename. If you renamed a column, it emits a drop plus an add and tells you to replace them:

```
review before applying:
  ! if products.title became name, replace the drop and add with migrate.RenameColumn to keep the data
  ! products.stock is NOT NULL without a default; existing rows need a value
```

That is a deliberate choice — silently destroying a column of data is worse than asking.

For prototyping, `migrate.Sync(handle, models)` runs GORM's `AutoMigrate` directly, with no files
and no ledger. Convenient in tests; do not point it at production.

### Tests

The suite lives in one root package, `tests`, and only touches exported API — a change that breaks
a caller breaks a test. One file per package under test, plus shared builders (`newTestApp`,
`newTestManager`, `newTestAuth`, `client`) in `helpers_test.go`.

```
tests/
  helpers_test.go   shared builders
  app_test.go       routing, middleware order, render, CSRF, settings wiring
  auth_test.go      passwords, the User entity, store rules, guards
  session_test.go   persistence, renewal, flashes, CSRF tokens, stores
  settings_test.go  defaults, validation, databases, env helpers
  admin_test.go     the admin portal end to end
```

The `_test.go` suffix is required: `go test` collects `TestXxx` only from files that carry it. Drop
the suffix and those tests stop running while `go test` still reports `ok` — a silent hole, not an
error.

```
go test ./tests/
go test ./tests/ -run Admin -v
go test -race ./tests/
```

## Settings

Settings are declared in code, in one place, and validated before the server starts. There is no
implicit configuration: `app.New()` panics if `settings.Configure` has not run.

```
$ go run .
panic: coyote/settings: improperly configured: no settings have been configured

	Coyote needs an explicit settings file. Create settings.go next to your main package:
	...
```

`Configure` takes functions that mutate a `Settings` value already populated with defaults, so you
only write what differs. Anything you leave alone keeps its default from `settings.Default()`.

```go
settings.Configure(func(s *settings.Settings) {
	s.Debug = false
	s.SecretKey = settings.Env("SECRET_KEY", "")
	s.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", []string{"example.com"})
	s.Sessions.Secure = true
	s.Sessions.Lifetime = 24 * time.Hour
})
```

Validation runs inside `Configure`, so mistakes surface at startup — all of them at once, not one
per restart:

```
panic: coyote/settings: improperly configured:
  - SecretKey is empty; set a random value of at least 32 characters
  - AllowedHosts is empty; with Debug disabled you must list the hosts this site serves
  - Sessions.SameSite must be "lax", "strict" or "none"
  - Admin.Prefix cannot be "/", it would take over every route
```

`Configure` may only be called once. Call it from `init()` in `settings.go` so it runs before
`main`, mirroring how Django loads its settings module first.

### Environments

Every project runs in one of three environments, and each carries a preset of sane defaults so you
are not hand-hardening production settings from scratch:

```go
settings.Configure(
	settings.Preset(settings.Env("APP_ENV", "development")),
	func(s *settings.Settings) {
		s.SecretKey = settings.Env("SECRET_KEY", "")
	},
)
```

A preset is just a mutator, so it composes with the ones after it — your own overrides always win.

| | development | staging and production |
| --- | --- | --- |
| `Debug` | on | off |
| `Logging` | `debug`, text | `info`, json |
| `Sessions.Secure` | off | on |
| `AllowedHosts` | localhost if unset | must be set |
| Template caching | reload every request | cached |
| Server timeouts | none | read 15s, write 30s, shutdown 20s |

Names are forgiving — `dev`, `local`, `stage`, `prod` and `live` all resolve — but an unrecognised
one is a configuration error rather than a silent fallback. Two safety rules are enforced:

```
coyote/settings: improperly configured: Debug must be off when Environment is production
coyote/settings: improperly configured: Environment "banana" is not recognised; use "development", "staging" or "production"
```

`s.IsDevelopment()`, `s.IsStaging()`, `s.IsProduction()` and `s.IsDeployed()` are available to
application code, and the active environment is logged at startup.

### Loading values from a file

Settings stay in code — that part does not change. What a file can supply is **values**, which your
`settings.go` then reads through the `Env` helpers. Secrets stay out of source control without
introducing a second place where configuration is decided.

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
anything without editing it. With several files, the first to define a key wins. A missing `.env` is
not an error; use `settings.RequiredDotEnv(path)` when it must exist.

The parser handles comments, blank lines, an optional `export` prefix, single quotes as literals,
double quotes with `\n` escapes, inline comments, and empty values. A malformed line is reported with
its line number rather than silently skipped.

Any other format is a matter of implementing one method, which is why no YAML or TOML dependency is
bundled:

```go
type Source interface {
	Name() string
	Values() (map[string]string, error)
}

settings.Load(myTOMLSource{path}, settings.DotEnv(".env"))
```

`settings.Map` is a built-in source, handy in tests.

### Every setting and its default

| Setting | Default | Notes |
| --- | --- | --- |
| `Debug` | `false` | Enables template reload, verbose render errors, relaxed host checks |
| `Environment` | `development` | `development`, `staging` or `production`; see presets below |
| `BaseDir` | working directory | Root that relative paths resolve against |
| `Databases` | one SQLite entry at `<BaseDir>/coyote.db` | First entry is the default connection |
| `SecretKey` | none | Required. Under `Debug` an ephemeral key is generated with a warning |
| `AllowedHosts` | none | Required unless `Debug`. `"*"` allows any, `".example.com"` matches subdomains |
| `Server.Host` | `127.0.0.1` | |
| `Server.Port` | `8000` | |
| `Server.ReadTimeout` | none | |
| `Server.WriteTimeout` | none | |
| `Server.IdleTimeout` | `2m` | |
| `Server.ReadHeaderTimeout` | `10s` | |
| `Server.ShutdownTimeout` | `10s` | Grace period on SIGINT/SIGTERM |
| `Server.TLS.CertFile` / `.KeyFile` | none | Both set means serve HTTPS |
| `Server.TLS.MinVersion` | TLS 1.2 | |
| `Server.TLS.HSTS` | none | Adds `Strict-Transport-Security`; requires TLS |
| `Server.TLS.Config` | none | A `*tls.Config` used as-is, for mTLS or your own ACME manager |
| `Server.TLS.Autocert` | `false` | Obtain and renew certificates from Let's Encrypt |
| `Server.TLS.AcceptTOS` | `false` | Required with `Autocert`; agrees to the authority's terms |
| `Server.TLS.CacheDir` | `certs` | Where issued certificates are stored |
| `Server.TLS.Staging` | `false` | Use the ACME staging directory while testing |
| `Server.Configure` | none | `func(*http.Server)` hook called before listening |
| `Sessions.CookieName` | `coyote_session` | |
| `Sessions.Lifetime` | `12h` | |
| `Sessions.Rolling` | `false` | Extend the deadline on every request |
| `Sessions.Secure` | `false` | Set true behind HTTPS |
| `Sessions.HTTPOnly` | `true` | |
| `Sessions.SameSite` | `lax` | `lax`, `strict`, or `none` (requires `Secure`) |
| `Sessions.Path` | `/` | |
| `Sessions.Domain` | none | |
| `Sessions.CleanupInterval` | `5m` | Expired-session sweep |
| `Sessions.Store` | `MemoryStore` | Any `session.Store` |
| `Auth.LoginURL` | `/admin/login` | Where guards send anonymous visitors |
| `Auth.PasswordMinLength` | `8` | |
| `Auth.PBKDF2Iterations` | `600000` | Lower it in tests to keep them fast |
| `Auth.UserStore` | database-backed | Any `auth.Store`; falls back to memory with no database |
| `Templates.FS` | none | An `fs.FS`, usually from `go:embed` |
| `Templates.Dir` | none | A directory path instead of an `fs.FS`; mutually exclusive with `FS` |
| `Templates.Layout` | `layouts/base.html` | |
| `Templates.Shared` | `layouts/*.html`, `partials/*.html` | Parsed into every page |
| `Templates.Funcs` | none | Extra template functions |
| `Static.URL` | `/static/` | |
| `Static.FS` / `Static.Dir` | none | Static files are served only when one is set |
| `Admin.Prefix` | `/admin` | |
| `Admin.SiteName` | `Coyote administration` | |
| `Admin.Tagline` | none | |
| `Logging.Level` | `info` | `debug`, `info`, `warn`, `error` |
| `Logging.Format` | `text` | `text` or `json` |
| `Logging.Logger` | none | Supply your own `*slog.Logger` |

Reading settings from the environment is done with the helpers, in `settings.go`, where you can
see it: `settings.Env`, `EnvBool`, `EnvInt`, `EnvDuration`, `EnvList`. The framework itself never
reads the environment.

At runtime the resolved settings are available as `a.Settings`, and the admin portal renders
them nowhere — the settings page was removed, since the portal is for your users, not for
inspecting the deployment.

### Databases

`Databases` is a list, and the first entry is the default connection. You get a SQLite database
without configuring anything:

```go
settings.Default().Databases
// []Database{{Alias: "default", Engine: "sqlite", Name: "<BaseDir>/coyote.db"}}
```

Relative SQLite paths resolve against `BaseDir`, so the file lands in the project folder. Absolute
paths and `:memory:` are left alone. `BaseDir` defaults to the working directory at startup — Go
has no `__file__`, so if you need it pinned regardless of where the binary is launched from, set it
explicitly.

Add connections by replacing the list. Keep the connection you want as the default first:

```go
settings.Configure(func(s *settings.Settings) {
	s.Databases = []settings.Database{
		{Engine: settings.Postgres, Name: "shop", Host: "db.internal",
			User: "app", Password: settings.Env("DB_PASSWORD", ""),
			Options: map[string]string{"sslmode": "require"}},
		{Alias: "cache", Engine: settings.SQLite, Name: "cache.db"},
	}
})
```

| Field | Notes |
| --- | --- |
| `Alias` | Blank becomes `default` for the first entry, then `db1`, `db2`… Must be unique |
| `Engine` | `sqlite`, `postgres`, or `mysql`. Blank defaults to `sqlite` |
| `Name` | SQLite file path, or the database name for a server engine |
| `Host` / `Port` | Server engines only; port defaults to 5432 or 3306 |
| `User` / `Password` | Server engines only; rejected on SQLite so mistakes are caught early |
| `Options` | Extra DSN parameters |
| `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime` | Pool tuning; zero means the driver default |

Accessors: `s.Database()` returns the default (first) connection, `s.DatabaseByAlias("cache")`
looks one up, `db.DSN()` builds the connection string, and `db.Redacted()` masks the password for
display. `settings.SQLiteDatabase(alias, name)` is a shorthand constructor.

Validation catches an empty list, unknown engines, duplicate aliases, a missing SQLite path,
credentials set on SQLite, a server engine without a host, and negative pool values.

## Routing

Patterns are passed straight to `net/http.ServeMux`, so Go 1.22 method and wildcard syntax
works as-is.

```go
a.Get("/items/{id}", show)
a.Post("/items", create)
a.Any("/webhook", hook)

api := a.Group("/api", requireToken)
api.Get("/status", status)

a.Mount("/legacy", someOtherHandler)
a.Static("/static/", staticFS)   // usually unnecessary: set Static.FS in settings instead
```

`a.Use(mw)` adds middleware for every request. Middleware passed to `Group` applies to that
group, and a trailing argument on a route applies to that route only.

```go
a.Post("/notes", createNote, a.CSRF)
```

`a.Routes()` returns the registered route table. The router is usable on its own — `router.New()`
gives you the same dispatch without the rest of the framework.

### Named routes

Every registration returns the route, so a name is one chained call. Nothing forces you to name a
route; do it where you want to build the URL later.

```go
a.Get("/{$}", home).Named("home")
a.Get("/posts/{id}", show).Named("post.detail")
a.Get("/files/{path...}", serve).Named("file")
```

Reverse them from Go, or from a template with the built-in `url` function:

```go
a.Reverse("post.detail", 42)        // "/posts/42", error
a.MustReverse("post.detail", 42)    // "/posts/42", panics
```

```html
<a href="{{url "post.detail" .ID}}">read</a>
```

Values fill the wildcards in order and are path-escaped, except `{path...}` which keeps its slashes.
A wrong name, a missing value, an empty value or too many values is an error, and in a template that
fails the render rather than emitting a broken link. Naming two routes the same panics at
registration, where you will see it immediately.

Group prefixes are included, so a route registered as `/status/{code}` inside `a.Group("/api")`
reverses to `/api/status/200`.

### HTTPS

Point `Server.TLS` at a certificate and key and `Run` serves over TLS:

```go
settings.Configure(func(s *settings.Settings) {
	s.Server.TLS.CertFile = settings.Env("TLS_CERT", "")
	s.Server.TLS.KeyFile = settings.Env("TLS_KEY", "")
	s.Server.TLS.HSTS = 30 * 24 * time.Hour
})
```

```
INFO coyote listening url=https://127.0.0.1:8443 environment=production tls=true
```

**HTTP/2 comes with it.** Go negotiates h2 over TLS automatically — no setting, no dependency.

`MinVersion` defaults to TLS 1.2. For anything the framework does not model there is
`Server.TLS.Config`, a `*tls.Config` used as-is (mTLS, a custom cipher list, your own ACME manager),
and `Server.Configure func(*http.Server)`, called just before listening — that is where h2c goes if
you need cleartext HTTP/2 behind a proxy.

### Certificates from Let's Encrypt

Coyote can obtain and renew certificates itself, so a bare VM needs no reverse proxy:

```go
s.Server.TLS.Autocert  = true
s.Server.TLS.AcceptTOS = true
s.Server.TLS.Staging   = settings.EnvBool("ACME_STAGING", false)
```

**Hosts come from `AllowedHosts`** — the names you already declare are the names certificates are
issued for, so there is no second list to keep in step. Certificates are cached in
`Server.TLS.CacheDir` (`certs/` under `BaseDir` by default); keep that directory across deploys or
you will re-issue on every restart and meet the rate limits.

Validation refuses the combinations that would otherwise fail at runtime, in the dark:

```
coyote/settings: improperly configured:
  - Server.TLS.Autocert needs Server.TLS.AcceptTOS set to true; issuing a certificate means agreeing to the certificate authority's terms of service
  - Server.TLS.Autocert cannot use the "*" host; a certificate authority needs real host names
  - Server.TLS.Autocert validates over TLS on port 443, so Server.Port must be 443; for anything else supply your own Server.TLS.Config
```

Three things worth knowing:

- **`AcceptTOS` is deliberately explicit.** Issuing a certificate means agreeing to the authority's
  terms, and the framework will not do that for you behind a default.
- **Validation happens over TLS on port 443** (TLS-ALPN-01), so no second listener on :80 is needed —
  but the port must be 443 and reachable from the internet.
- **Use `Staging` while you iterate.** Let's Encrypt's production rate limits are strict and a deploy
  loop will hit them; the staging directory issues untrusted certificates without the limits.

Two middlewares support a TLS deployment, both aware of `X-Forwarded-Proto` so they work behind a
terminating proxy:

```go
a.Use(middleware.RequireHTTPS)   // redirect plain HTTP to https
```

`middleware.HSTS` is added for you when `Server.TLS.HSTS` is set, and only emits the header on
connections that are actually secure. Setting HSTS without TLS is a configuration error — a browser
that sees it will refuse plain HTTP to your host for the whole duration.

One `net/http` behaviour to know: a pattern ending in `/` matches a whole subtree, so `Get("/", …)`
answers *every* unmatched path and nothing ever 404s. Use `{$}` when you mean the exact path:

```go
a.Get("/{$}", home)      // only "/"
a.Get("/docs/", docs)    // "/docs/" and everything under it
```

## Templates

Pages are rendered through a layout. A layout and its partials are parsed into every page, so
`{{define "content"}}` in a page fills `{{block "content" .}}` in the layout.

```
templates/
  layouts/base.html     ← {{define "base.html"}} ... {{block "content" .}}{{end}} ... {{end}}
  partials/nav.html     ← {{define "nav.html"}} ... {{end}}
  pages/home.html       ← {{define "content"}} ... {{end}}
```

```go
a.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})
```

`Templates.Shared` defaults to `layouts/*.html` and `partials/*.html`. Every render is given
`.User`, `.Session`, `.CSRFToken`, `.Flashes`, `.Path`, `.Request`, `.Debug`, and `.Version` on top
of your own data. When `Debug` is true templates are re-parsed on each request.

## Sessions

Sessions are stored server-side; the browser only ever holds an opaque id in an `HttpOnly`
cookie.

```go
sess := session.FromRequest(r)
sess.Set("cart", items)
count := sess.GetInt("visits")
sess.Pop("one_time_value")
```

Flash messages survive exactly one redirect and drain when read:

```go
view.Success(r, "Saved.")
view.Redirect(w, r, "/notes")
```

`core/view` also has `Flash`, `Error`, `Warning`, `Info`, and `RedirectPermanent`.

The cookie is written lazily, just before the response headers go out, so a session mutated
anywhere in the handler chain is still persisted correctly. `SessionRolling: true` extends the
deadline on every request.

### Where sessions live

```go
s.Sessions.Backend = settings.SessionsInDB
```

`memory` is the default: fast, zero setup, and everyone is signed out when the process restarts.
`database` stores sessions in a `sessions` table so they survive restarts and are shared across
instances. The table is a registered model, so `makemigrations` generates it like any other.

Values are encoded with **gob**, not JSON. That matters: JSON would turn every number into a
`float64`, so `GetInt` would silently return `0` and `Flashes()` would break. Gob keeps the Go types
intact. Custom types stored in a session need registering once:

```go
session.RegisterValue(MyType{})
```

One thing to weigh before switching: `Sessions.Rolling` marks the session modified on every request,
so rolling expiry plus the database backend means a write per request. Leave `Rolling` off unless
you need it.

Either way you can supply your own `session.Store` (`Load`, `Save`, `Delete`, plus `Count`, `All`
and `DeleteByUserID` if you want the admin's session list to work).

## First run

A fresh install has no schema and no users. The framework tells you so on startup — a real check
against the ledger, not a first-boot flag:

```
$ go tool coyote start
WARN  this database has no schema yet and no migrations are declared; run: go tool coyote makemigrations && go tool coyote migrate
INFO  coyote listening addr=127.0.0.1:8081
```

Once migrations exist but have not been applied it says so, and once applied it checks whether
anyone can actually sign in:

```
WARN  no migrations have been applied to this database; nothing will work until you run: go tool coyote migrate pending=1
WARN  migrations are pending; run: go tool coyote migrate pending=1 applied=2
WARN  there are no users yet, so nobody can sign in; run: go tool coyote createsuperadmin
INFO  migrations up to date applied=3
```

The server still starts in every case — an unreachable database downgrades to a warning rather
than refusing to boot.

So the bootstrap sequence for a new project is:

```
go tool coyote makemigrations --name=initial
go tool coyote migrate
go tool coyote createsuperadmin
go tool coyote start
```

### createsuperadmin

```
go tool coyote createsuperadmin
go tool coyote createsuperadmin --username=root --email=root@site.com --password=secret
COYOTE_SUPERADMIN_PASSWORD=secret go tool coyote createsuperadmin --username=root
```

Flags win, then the `COYOTE_SUPERADMIN_*` environment variables, then an interactive prompt for
whatever is still missing. The password is **echoed** while you type — use the environment variable
form in scripts and shared terminals. The command refuses to run before the `users` table exists:

```
coyote/cli: the users table does not exist yet; run: go tool coyote makemigrations && go tool coyote migrate
```

Users are created through the same service the admin portal uses, so the password is hashed by one
code path.

## The user entity

`auth.User` is the ready-made entity you get on install. GORM derives the columns from the field
names (`FirstName` becomes `first_name`), so tags only appear where behaviour is needed:

```go
type User struct {
	ID           string `gorm:"primaryKey;size:64"`
	FirstName    string
	LastName     string
	Email        string `gorm:"index;size:320"`
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Password     string `gorm:"not null"`
	IsActive     bool   `gorm:"index;default:true"`
	IsSuperadmin bool   `gorm:"index"`
	LastLoginAt  time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

`Password` always holds a PBKDF2 hash — the service is the only thing that writes it, and
`VerifyPassword` rejects anything that is not in hash format, so a plain string assigned by mistake
fails closed rather than authenticating. `HasUsablePassword` reports whether the stored value is a
recognised hash.

Derived helpers: `FullName()`, `DisplayName()` (falls back to the username), `Initials()`,
`HasLoggedIn()`, `Clone()`, `Validate()`.

Creating users goes through the service so hashing and defaults are never skipped. New accounts are
active, and not superadmin unless asked:

```go
user, err := a.Auth.CreateUser(auth.NewUser{
	Username:  "jane",
	Email:     "jane@example.com",
	FirstName: "Jane",
	LastName:  "Doe",
	Password:  "supersecret",
})

root, err := a.Auth.CreateSuperadmin("root", "root@example.com", "supersecret")
```

`IsSuperadmin` is the only role flag: it grants full permissions and access to the admin portal.
There is no separate staff tier — add your own field if you want an intermediate role.

Store rules: usernames and emails are unique case-insensitively, a blank email is allowed and does
not collide, and the last active superadmin cannot be deleted, demoted, or disabled.

### Replacing or extending it

Two seams exist today. Embed the entity when you only need extra fields alongside it:

```go
type Employee struct {
	auth.User
	Department string
	ManagerID  string
}
```

Or implement `auth.Store` over your own table and set `Auth.UserStore` in settings, which is what
the persistence layer will use. A first-class swappable user model — the equivalent of Django's
`AUTH_USER_MODEL` — arrives with the database layer; today the framework's own code still works in
terms of `*auth.User`.

## Authentication

Passwords are hashed with PBKDF2-SHA256 using `Auth.PBKDF2Iterations` (600,000 by default) and a
16-byte random salt, in the Django-style `pbkdf2_sha256$iterations$salt$hash` format.

```go
user, err := a.Auth.Authenticate(username, password)
if err == nil {
	a.Auth.Login(r, user)   // rotates the session id
}
a.Auth.Logout(r)
a.Auth.CurrentUser(r)
```

Guards redirect anonymous visitors to the login page with a `?next=` parameter and return 403
for signed-in users who lack the role:

```go
login := a.Settings.Auth.LoginURL
a.Group("/me", a.Auth.RequireLogin(login))
a.Group("/root", a.Auth.RequireSuperadmin(login))
```

## Rate limiting

```go
s.Security.RateLimit = settings.RateLimit{
	Requests: 60,
	Window:   time.Minute,
	Burst:    100,
}
```

A token bucket per client: `Requests` per `Window` is the sustained rate, `Burst` the short-term
ceiling. Over the limit returns 429 with `Retry-After`, and every response carries `RateLimit-Limit`
and `RateLimit-Remaining`.

Clients are identified by IP. `X-Forwarded-For` is **ignored unless `TrustProxy` is set**, because
without a proxy in front, anyone can send that header and mint themselves a fresh budget on every
request. Set it only when a proxy you control rewrites the header.

For anything other than an IP — an API key, a tenant, an account — supply your own key:

```go
a.Use(middleware.RateLimitBy(policy, func(r *http.Request) string {
	return r.Header.Get("X-Api-Key")
}))
```

Buckets live in memory and stale ones are swept, so the map does not grow with every unique visitor.
That also means each process has its own budget: behind several instances the effective limit is
per instance until a shared backend exists.

## CORS

Off until you list origins. A request with no `Origin` header is left alone entirely.

```go
s.Security.CORS = settings.CORS{
	Origins:          []string{"https://app.example.com"},
	AllowCredentials: true,
	MaxAge:           10 * time.Minute,
}
```

Methods and headers have sensible defaults, preflights are answered with 204, and `Vary` is set on
`Origin` (plus the request-method and request-header names on preflights) so caches cannot serve one
origin's response to another.

Two rules are enforced at startup rather than discovered in a browser console:

```
Security.CORS cannot combine the "*" origin with AllowCredentials; browsers reject that pairing, so list the origins you mean
Security.CORS origin "app.example.com" needs a scheme, for example https://app.example.com
```

Note what CORS is not: an unlisted origin still gets its response for a simple `GET`, because the
*browser* enforces the block, not the server. Preflights are refused outright. If you need the
server to reject the request itself, that is authentication's job, not CORS's.

## Compression

```go
s.Security.Compress = true
s.Security.CompressLevel = 6   // 1-9, or 0 for the default
```

Gzip is applied only where it helps. Responses are left alone when the client did not ask for it,
when the body is under 1 KB, when something already set `Content-Encoding`, when the status carries
no body, and for content types that do not shrink — images, video, audio, zip, PDF. `Vary:
Accept-Encoding` is always set, including on uncompressed responses, so a cache cannot hand a
gzipped body to a client that cannot read it. `Content-Length` is dropped once the body is
compressed, and `Flush` still reaches the client, so streaming responses keep working.

## Content Security Policy

Off by default, because a policy that breaks your pages is worse than none. Turn it on with a policy
string, or start from the strict default:

```go
s.Security.CSP = settings.DefaultCSP
s.Security.CSPReportOnly = true    // observe first, enforce later
```

`DefaultCSP` allows nothing but your own origin, forbids objects and framing, and carries no
`unsafe-inline`. Inline styles and scripts are handled with a **nonce** instead: write `{nonce}`
anywhere in the policy and each response gets a fresh one, reachable in templates as `.Nonce`.

```html
<style nonce="{{.Nonce}}"> … </style>
```

The admin portal's own inline stylesheet already carries the nonce, so it keeps working under the
strict default. `middleware.NonceFrom(r.Context())` gives you the same value in a handler.

## CSRF

`a.CSRF` rejects unsafe methods without a valid token, read from the `csrf_token` form field
or the `X-CSRF-Token` header. The token is per-session and compared in constant time. Put
`{{.CSRFToken}}` in every form:

```html
<form method="post" action="/notes">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
</form>
```

The admin portal applies it to all of its own routes.

## Admin portal

Mounting takes no options — it reads `Admin.Prefix`, `Admin.SiteName`, and `Admin.Tagline` from
settings:

```go
portal := admin.Mount(a)
```

Provides a login screen, dashboard, user management (create, edit, search, delete, password
reset), and an active session list with revoke. Access requires `IsSuperadmin`. Editing your own
account locks the role and status fields, self-deletion is refused, the last active superadmin
cannot be removed, and changing a password revokes that user's other sessions.

## Managed resources

Register a model and the portal grows a full CRUD section for it. You write no routes, no handlers,
and no templates — the framework derives everything from the entity.

```go
type productResource struct{}

func (productResource) Entity() any { return Product{} }

portal.MustManage(productResource{})
```

That produces, in plural form:

```
GET  /admin/products              list, 4 columns plus an actions column
GET  /admin/products/new          empty form
POST /admin/products/new          create
GET  /admin/products/{id}         detail form
POST /admin/products/{id}         update
POST /admin/products/{id}/delete  delete
```

The section also appears in the sidebar. Routes are generated per resource at registration time
rather than from a single `/{resource}` wildcard, because `net/http` rejects
`GET /admin/{resource}/new` as ambiguous against `GET /admin/users/{id}` — both match
`/admin/users/new` and neither is more specific. Call `Manage` after `Mount` and before `Run`.

### What the form derives

Inputs come from the column types, one input per column:

| Go type | Input |
| --- | --- |
| `string` | `text`, or `password` for password-ish names, `email` for `email` |
| `int`, `uint` | `number` |
| `float64` | `number` with `step="any"` |
| `bool` | `checkbox` |
| `time.Time` | `datetime-local` |

A `NOT NULL` column with no default is **mandatory**; nullable columns are optional. `size` becomes
`maxlength` and is validated server-side too. The primary key is never an input: it is generated by
the framework (a UUID for string keys, left to the database for autoincrement keys) and rendered
read-only on the detail page. `created_at` and `updated_at` are framework-managed and never shown as
editable.

### Customising through interfaces

`Resource` is the only interface you must satisfy. Everything else is opt-in — implement just the
ones you need, and the framework detects them:

```go
type Resource interface { Entity() any }

type Labelled   interface { Label() string }          // singular label
type Pluralised interface { PluralLabel() string }    // heading and sidebar
type Routed     interface { Slug() string }           // URL segment
type Listed     interface { ListColumns() []string }  // which columns in the list
type Concealed  interface { HiddenColumns() []string }// never render these
type Sorted     interface { DefaultOrder() string }   // list ordering
type Guarded    interface { ReadOnly() bool }         // browse only, no writes
```

```go
func (productResource) ListColumns() []string   { return []string{"name", "sku", "price"} }
func (productResource) HiddenColumns() []string { return []string{"internal_note"} }
```

`Manage` returns an error rather than panicking for the cases you can get wrong: `ErrReservedSlug`
(the portal already owns `users`, `sessions`, `login`, `logout`), `ErrDuplicateSlug`, and
`ErrNotMountedYet`.

### The pieces behind it

```
core/model    Schema, Field, Kind, Describe — introspection from gorm tags
core/model    Store interface, Record, Query — the data-access contract
core/store    the gorm-backed Store implementation
lib/id        UUID generation
contrib/admin Resource interfaces, registry, generic handlers, two templates
```

The admin depends only on `model.Store`, never on `core/store`, so a different backend is a matter of
supplying another implementation. One list template and one form template serve every resource.

## Security defaults

- Session ids are 256 bits from `crypto/rand`; the id rotates on login.
- Cookies are `HttpOnly` and `SameSite=Lax` by default; set `Sessions.Secure` behind HTTPS.
- `AllowedHosts` is enforced on every request: an unlisted `Host` header gets a 400.
- Settings are validated at startup, so an unsafe production config fails before serving traffic.
- `X-Content-Type-Options`, `X-Frame-Options`, and `Referrer-Policy` are set on every response.
- Every request gets an id, returned as `X-Request-Id` and attached to its access log line.
- A Content Security Policy is available with per-request nonces; off until you set `Security.CSP`.
- Login on an unknown username still runs a hash to even out response timing.
- `?next=` redirect targets are restricted to same-origin paths.
- Panics are recovered, logged with a stack trace, and returned as a plain 500.

## Testing

```
go test ./tests/
go test -race ./tests/
```

## Not here yet

`session.Store` is still in-memory, so restarting the server signs everyone out. `SecretKey` is
validated and surfaced but unused, reserved for signed cookies and tokens. Also missing: a
form/validation layer, down migrations, and project scaffolding.
