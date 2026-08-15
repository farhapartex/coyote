<p align="center">
  <img src="./assets/coyote.png" alt="Coyote" width="220">
</p>

<h1 align="center">coyote</h1>

<p align="center">
  A session-first web framework for Go, shaped like Django but sized for the standard library.
</p>

---

Routing, sessions, authentication, templates, middleware, and the admin portal are pure standard
library. Persistence uses GORM with a cgo-free SQLite driver, so `go get` is the whole install and
`CGO_ENABLED=0` still builds a static binary.

Configuration is explicit: every project declares a `settings.go`, and the app refuses to start
without one. Settings are validated before the server listens, so an unsafe production config fails
at startup instead of at 3am.

```go
func main() {
	a := app.New()

	admin.Mount(a)

	a.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})
	}).Named("home")

	log.Fatal(a.Run())
}
```

## What comes with it

- **Admin portal** — login, dashboard, user management, session revoke. Register a model and it
  grows a full CRUD section with no routes, handlers, or templates from you.
- **Sessions** — three backends: in memory, in the database, or sealed in the cookie with AES-GCM.
- **Authentication** — a user entity, PBKDF2 passwords, sign-in, and route guards.
- **Migrations** — versioned, written in Go rather than SQL, applied in a transaction, checksummed
  so an edited migration is caught rather than silently skipped.
- **Security by default** — allowed hosts, secure headers, CSRF, CSP with per-request nonces, rate
  limiting, CORS, gzip, HSTS, and Let's Encrypt certificates.
- **One CLI** — `makemigrations`, `migrate`, `createsuperadmin`, `start`, pinned to your project as
  a Go tool.

## Install

```
go get github.com/farhapartex/coyote
go get -tool github.com/farhapartex/coyote/cmd/coyote
```

Requires Go 1.25 or newer. GORM and the SQLite driver come with it — nothing to install separately.

## Documentation

Start with the [quick start](guide/02-quickstart.md), or browse the full
[documentation contents](guide/README.md).

| | | |
| --- | --- | --- |
| **Getting started** | [Installation](guide/01-installation.md) · [Quick start](guide/02-quickstart.md) · [Project layout](guide/03-project-layout.md) | |
| **Configuration** | [Settings](guide/04-settings.md) | |
| **Requests** | [Routing](guide/05-routing.md) · [Named routes](guide/06-named-routes.md) · [Views](guide/07-views.md) · [Templates](guide/08-templates.md) · [Static files](guide/09-static-files.md) | |
| **Data** | [Models](guide/10-models.md) · [Database](guide/11-database.md) · [Migrations](guide/12-migrations.md) | |
| **Users** | [Sessions](guide/13-sessions.md) · [Authentication](guide/14-authentication.md) · [Admin portal](guide/15-admin.md) | |
| **Security** | [Middleware](guide/16-middleware.md) · [Headers and CSP](guide/17-security-headers.md) · [CSRF](guide/18-csrf.md) · [Rate limiting](guide/19-rate-limiting.md) · [CORS](guide/20-cors.md) · [Compression](guide/21-compression.md) · [HTTPS](guide/22-https.md) | |
| **Operations** | [CLI](guide/23-cli.md) · [First run](guide/24-first-run.md) · [Deployment](guide/25-deployment.md) · [Testing](guide/26-testing.md) · [Architecture](guide/27-architecture.md) | |

## Running the example

```
cd example
go run .
```

Serves on `127.0.0.1:8000` with a small site, two managed models, and the admin portal. It reads
`PORT`, `HOST`, `DEBUG`, `SECRET_KEY`, `ALLOWED_HOSTS` and `DB_NAME` from the environment, so
`PORT=9000 go run .` works.

## Tests

```
go test ./tests/
go test -race ./tests/
```

## Status

Coyote is in active development on phase 1. Working today: settings, routing, sessions,
authentication, templates, migrations, the admin portal, and the security middleware above.

Not here yet: a form and validation layer, down migrations, a swappable user model, project
scaffolding, caching, internationalisation, and email.
