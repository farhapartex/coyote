# Coyote

A session-first web framework for Go, shaped like Django but sized for the standard library.
Routing, sessions, auth, templates, and the admin portal are pure standard library; persistence
uses GORM with a cgo-free SQLite driver.

Configuration is explicit: every project declares a `settings.go`, and the app refuses to start
without one. It ships with settings, sessions, authentication, HTML template rendering, and a
working admin portal.

Authentication state lives in the session; everything else is persisted, with a SQLite database
configured out of the box. `go tool coyote migrate` builds the schema from your entities. The
`auth.Store` and `session.Store` implementations are still the in-memory ones — wiring them onto
GORM is the next step.

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

	"github.com/farhapartex/coyote/admin"
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

## Layout

Coyote follows MVT. `core/` holds the framework; `contrib/` holds tooling built on top of it;
`admin/`, `cmd/`, `tests/`, and `example/` sit beside them.

```
core/
  app/          the application object: wiring, lifecycle, render entry point
  router/       URL dispatch — groups, method helpers, static, route table
  view/         view helpers — template data, redirects, flash messages      (V)
  template/     the html/template engine with layouts and partials          (T)
  model/        model registry: which entities the schema is built from      (M)
  auth/         the User entity, its store, passwords, login guards          (M)
  db/           GORM connection, pool tuning, pragmas, slog bridge
  session/      Session, Store interface, MemoryStore, cookie manager
  middleware/   logging, recovery, allowed hosts, secure headers, CSRF
  settings/     the settings type, defaults, validation, env helpers
contrib/
  cli/          the management commands (start, migrate) and their registry
  migrate/      schema planning and application through GORM AutoMigrate
cmd/coyote/     the `go tool coyote` front end
admin/          the built-in admin portal (embedded templates)
tests/          the whole test suite, one package, black box
example/        a small site using the framework
```

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

Nothing under `core/` imports `admin`, `tests`, or `example`, and only `core/app` imports
`contrib/cli` — commands reach the application through an interface, never the concrete type.

### Commands

Coyote ships a management command, registered as a Go tool so it is versioned with your project:

```
go get -tool github.com/farhapartex/coyote/cmd/coyote

go tool coyote start                # serve on the port from settings.go
go tool coyote start --port=5000     # override it for this run
go tool coyote migrate               # apply pending schema changes
go tool coyote help
```

`go coyote start` is not possible — the `go` command cannot be extended with new subcommands.
`go tool` is the closest supported form. The tool builds and runs the main package in the current
directory, so your `settings.go` is the one that applies; it passes the subcommand through
`COYOTE_COMMAND` and any overrides through `COYOTE_PORT` / `COYOTE_HOST`. Your `main.go` needs
nothing beyond `app.New()` and `Run()` — `Run` dispatches to the requested command.

`migrate` refuses to do anything unless the server is already listening on the configured address:

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

Schema comes from your entities through GORM's `AutoMigrate` — there is no hand-written DDL and no
migration files to keep in sync. `contrib/migrate` adds the part GORM lacks: it inspects the live
database first, so it can tell you *what* is about to change and report each step.

```go
a.RegisterModel(model.Of(Post{}), model.Named("legacy_orders", Order{}))
```

`auth.User` is registered for you. What is detected today: missing tables, and columns added to an
existing table. Renames, drops, and type changes are not — GORM's `AutoMigrate` is additive by
design, and destructive operations should not happen implicitly.

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

### Every setting and its default

| Setting | Default | Notes |
| --- | --- | --- |
| `Debug` | `false` | Enables template reload, verbose render errors, relaxed host checks |
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
| `Auth.UserStore` | `MemoryStore` | Any `auth.Store` |
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
them at `/admin/settings` with `SecretKey` and database passwords redacted.

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

`a.Routes()` returns the registered route table, which the admin portal displays. The router is
usable on its own — `router.New()` gives you the same dispatch without the rest of the framework.

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

Swap `MemoryStore` for your own `session.Store` (`Load`, `Save`, `Delete`) when a database
arrives.

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
reset), an active session list with revoke, the route table, and a read-only settings page.
Access requires `IsSuperadmin`.
Editing your own account locks the role and status fields, self-deletion is refused, the last
active superadmin cannot be removed, and changing a password revokes that user's other sessions.

Applications can contribute their own pages:

```go
portal.Register(admin.Section{
	Name:        "Reports",
	Slug:        "reports",
	Description: "Monthly exports",
	Handler:     reportsHandler,
})
```

Sections appear in the nav and on the dashboard, mounted at `/admin/s/{slug}/` behind the same
staff guard.

## Security defaults

- Session ids are 256 bits from `crypto/rand`; the id rotates on login.
- Cookies are `HttpOnly` and `SameSite=Lax` by default; set `Sessions.Secure` behind HTTPS.
- `AllowedHosts` is enforced on every request: an unlisted `Host` header gets a 400.
- Settings are validated at startup, so an unsafe production config fails before serving traffic.
- `X-Content-Type-Options`, `X-Frame-Options`, and `Referrer-Policy` are set on every response.
- Login on an unknown username still runs a hash to even out response timing.
- `?next=` redirect targets are restricted to same-origin paths.
- Panics are recovered, logged with a stack trace, and returned as a plain 500.

## Testing

```
go test ./tests/
go test -race ./tests/
```

## Not here yet

The `auth.Store` and `session.Store` implementations are still in-memory, so migrated tables are
not yet read or written by the running app — that wiring is next. `SecretKey` is validated and
surfaced but unused, reserved for signed cookies and tokens. Also missing: a form/validation layer,
destructive migration handling, Postgres and MySQL drivers, a static asset pipeline, and project
scaffolding.
