# Coyote

A session-first web framework for Go, shaped like Django but sized for the standard library.
Zero third-party dependencies — everything is built on `net/http`, `html/template`, and `crypto/*`.

Configuration is explicit: every project declares a `settings.go`, and the app refuses to start
without one. It ships with settings, sessions, authentication, HTML template rendering, and a
working admin portal.

The intended split is that authentication state lives in the session and everything else is
persisted, with a SQLite database configured out of the box. `Databases` is settled (see below);
the connection layer built on top of it is not written yet, so users and sessions are still held
in memory behind their `Store` interfaces.

## Install

```
go get github.com/farhapartex/coyote
```

Requires Go 1.24 or newer (`crypto/pbkdf2`).

## Quick start

Every Coyote project needs two files. First `settings.go`, which is mandatory — the app refuses
to start without it:

```go
package main

import (
	"embed"
	"io/fs"

	"github.com/farhapartex/coyote/settings"
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

	"github.com/farhapartex/coyote"
	"github.com/farhapartex/coyote/admin"
)

func main() {
	app := coyote.New()

	app.Auth.CreateUser("admin", "admin@example.com", "coyote123", true, true)
	admin.Mount(app)

	app.Get("/", func(w http.ResponseWriter, r *http.Request) {
		app.Render(w, r, "pages/home.html", coyote.Data{"Title": "Home"})
	})

	log.Fatal(app.Run())
}
```

Then open `http://localhost:8000/` and `http://localhost:8000/admin/`.

## Running the example

```
cd example
go run .
```

Serves on `127.0.0.1:8000`. `example/settings.go` reads `PORT`, `HOST`, `DEBUG`, `SECRET_KEY`, and
`ALLOWED_HOSTS` from the environment, so `PORT=9000 go run .` works. Two users are seeded:

| Username | Password    | Role      |
| -------- | ----------- | --------- |
| `admin`  | `coyote123` | superuser |
| `editor` | `coyote123` | staff     |

## Layout

```
coyote.go       App, render helpers, graceful run
router.go       ServeMux wrapper: groups, method helpers, route table
middleware.go   logging, recovery, allowed hosts, secure headers, CSRF
settings/       the settings type, defaults, validation, env helpers
session/        Session type, Store interface, MemoryStore, cookie manager
auth/           User, Store interface, PBKDF2 passwords, login guards
render/         html/template engine with layouts and partials
admin/          the built-in admin portal (embedded templates)
example/        a small site using the framework
```

## Settings

Settings are declared in code, in one place, and validated before the server starts. There is no
implicit configuration: `coyote.New()` panics if `settings.Configure` has not run.

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

At runtime the resolved settings are available as `app.Settings`, and the admin portal renders
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
app.Get("/items/{id}", show)
app.Post("/items", create)
app.Any("/webhook", hook)

api := app.Group("/api", requireToken)
api.Get("/status", status)

app.Mount("/legacy", someOtherHandler)
app.Static("/static/", staticFS)   // usually unnecessary: set Static.FS in settings instead
```

`app.Use(mw)` adds middleware for every request. Middleware passed to `Group` applies to that
group, and a trailing argument on a route applies to that route only.

```go
app.Post("/notes", createNote, app.CSRF)
```

`app.Routes()` returns the registered route table, which the admin portal displays.

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
app.Render(w, r, "pages/home.html", coyote.Data{"Title": "Home"})
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
coyote.Flash(r, "success", "Saved.")
coyote.Redirect(w, r, "/notes")
```

The cookie is written lazily, just before the response headers go out, so a session mutated
anywhere in the handler chain is still persisted correctly. `SessionRolling: true` extends the
deadline on every request.

Swap `MemoryStore` for your own `session.Store` (`Load`, `Save`, `Delete`) when a database
arrives.

## Authentication

Passwords are hashed with PBKDF2-SHA256 using `Auth.PBKDF2Iterations` (600,000 by default) and a
16-byte random salt, in the Django-style `pbkdf2_sha256$iterations$salt$hash` format.

```go
user, err := app.Auth.Authenticate(username, password)
if err == nil {
	app.Auth.Login(r, user)   // rotates the session id
}
app.Auth.Logout(r)
app.Auth.CurrentUser(r)
```

Guards redirect anonymous visitors to the login page with a `?next=` parameter and return 403
for signed-in users who lack the role:

```go
login := app.Settings.Auth.LoginURL
app.Group("/me", app.Auth.RequireLogin(login))
app.Group("/staff", app.Auth.RequireStaff(login))
app.Group("/root", app.Auth.RequireSuperuser(login))
```

Users carry `IsActive`, `IsStaff`, and `IsSuperuser`; superuser implies staff.

## CSRF

`app.CSRF` rejects unsafe methods without a valid token, read from the `csrf_token` form field
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
portal := admin.Mount(app)
```

Provides a login screen, dashboard, user management (create, edit, search, delete, password
reset), an active session list with revoke, the route table, and a read-only settings page.
Access requires `IsStaff`.
Editing your own account locks the role and status fields, self-deletion is refused, the last
active superuser cannot be removed, and changing a password revokes that user's other sessions.

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
go test ./...
go test -race ./...
```

## Not here yet

`Databases` is declared and validated, but nothing opens a connection yet: no driver, no `*sql.DB`,
no migrations, no ORM. `SecretKey` is likewise validated and surfaced but unused — reserved for
signed cookies and tokens. Also missing: a form/validation layer, a static asset pipeline, and CLI
scaffolding. Those come in later stages.
