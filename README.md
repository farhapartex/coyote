# Coyote

A session-first web framework for Go, shaped like Django but sized for the standard library.
Zero third-party dependencies — everything is built on `net/http`, `html/template`, and `crypto/*`.

Stage 1 gives you sessions, authentication, HTML template rendering, and a working admin
portal. There is no database layer yet: users and sessions live in memory behind interfaces,
so persistence can be dropped in later without touching the handlers.

## Install

```
go get github.com/farhapartex/coyote
```

Requires Go 1.24 or newer (`crypto/pbkdf2`).

## Quick start

```go
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/farhapartex/coyote"
	"github.com/farhapartex/coyote/admin"
)

//go:embed templates
var templateFS embed.FS

func main() {
	templates, _ := fs.Sub(templateFS, "templates")

	app := coyote.New(coyote.Config{
		Addr:      ":8000",
		Templates: templates,
		Layout:    "layouts/base.html",
		DevMode:   true,
	})

	app.Auth.CreateUser("admin", "admin@example.com", "coyote123", true, true)
	admin.Mount(app, admin.Options{SiteName: "My site admin"})

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

Serves on `:8000` (override with `ADDR=:9000`). Two users are seeded:

| Username | Password    | Role      |
| -------- | ----------- | --------- |
| `admin`  | `coyote123` | superuser |
| `editor` | `coyote123` | staff     |

## Layout

```
coyote.go       App, config, render helpers, graceful run
router.go       ServeMux wrapper: groups, method helpers, route table
middleware.go   logging, panic recovery, secure headers, CSRF
session/        Session type, Store interface, MemoryStore, cookie manager
auth/           User, Store interface, PBKDF2 passwords, login guards
render/         html/template engine with layouts and partials
admin/          the built-in admin portal (embedded templates)
example/        a small site using the framework
```

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
app.Static("/static/", staticFS)
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

`Shared` defaults to `layouts/*.html` and `partials/*.html`; override it with
`Config.SharedTemplates`. Every render is given `.User`, `.Session`, `.CSRFToken`, `.Flashes`,
`.Path`, `.Request`, and `.Version` on top of your own data. With `DevMode: true` templates are
re-parsed on each request.

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

Passwords are hashed with PBKDF2-SHA256, 600,000 iterations, a 16-byte random salt, in the
Django-style `pbkdf2_sha256$iterations$salt$hash` format.

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
app.Group("/me", app.Auth.RequireLogin("/admin/login"))
app.Group("/staff", app.Auth.RequireStaff("/admin/login"))
app.Group("/root", app.Auth.RequireSuperuser("/admin/login"))
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

```go
portal := admin.Mount(app, admin.Options{
	Prefix:   "/admin",
	SiteName: "My site admin",
	Tagline:  "internal tools",
})
```

Provides a login screen, dashboard, user management (create, edit, search, delete, password
reset), an active session list with revoke, and the route table. Access requires `IsStaff`.
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
- Cookies are `HttpOnly` and `SameSite=Lax` by default; set `SessionSecure: true` behind HTTPS.
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

No database or ORM, no migrations, no form/validation layer, no signed-cookie session backend,
no static asset pipeline, no CLI scaffolding. Those come in later stages.
