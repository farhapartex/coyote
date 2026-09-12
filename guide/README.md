<p align="center">
  <img src="assets/coyote.png" alt="Coyote" width="180">
</p>

<h1 align="center">coyote</h1>

<p align="center">
  A full-stack web framework for Go. Server-rendered, session-first, and built on the standard library.
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
- **Sessions** — three backends: in the database by default, in memory, or sealed in the cookie with
  AES-GCM.
- **Authentication** — a user entity, PBKDF2 passwords, sign-in, route guards, pluggable password
  rules, and login throttling on by default.
- **Permissions** — four permissions per model, bundled into roles, managed from the portal.
- **Forms and uploads** — struct or schema binding with validation, and file uploads that stage,
  sniff, and clean up after themselves.
- **Migrations** — versioned, written in Go rather than SQL, checksummed so an edited migration is
  caught rather than silently skipped, applied in a transaction wherever the engine has one, and
  behind an advisory lock so two deploying instances cannot race.
- **Security on by default** — CSRF on every route, login throttling, allowed hosts, secure headers,
  a request body cap, and a session cookie that a deployed environment refuses to send in clear.
  **Opt in** to CSP with per-request nonces, rate limiting, CORS, gzip, HSTS and Let's Encrypt
  certificates: each is one setting, and validation tells you when a combination is unsafe.
- **Caching** — one interface over memory, disk or Redis, with `Remember`, template fragments, whole
  pages and query results. The Redis client is standard library, so it costs no dependency.
- **Internationalisation** — gettext catalogs, plural rules read from each translator's own file,
  locale detection, and an admin portal that is already translated and right-to-left correct.
- **Background jobs** — a queue in your own database, so a job commits in the same transaction as the
  data it is about. Retries with backoff, a dead-letter state you can retry from the portal, interval
  scheduling, and workers either in the web process or as `coyote worker`. No broker, no dependency.
- **One CLI** — `makemigrations`, `migrate`, `createsuperadmin`, `start`, `worker`, pinned to your
  project as a Go tool.

## Install

```
go install github.com/farhapartex/coyote/cmd/coyote@latest

coyote new myshop
cd myshop
coyote start
```

`new` scaffolds a project that already builds — settings, routes, a layout, a generated `SecretKey`
in a git-ignored `.env` — then pulls the framework and tidies. Nothing to clone.

Requires Go 1.25 or newer. GORM and the SQLite driver come with it; there is nothing else to
install.

## Documentation

Each page covers one topic and nothing else. Read them in order to learn the framework, or jump
straight to the one you need.

## Getting started

| | |
| --- | --- |
| [1. Installation](01-installation.md) | Requirements, `go get`, the management command |
| [2. Quick start](02-quickstart.md) | A working site with an admin portal, from an empty directory |
| [3. Project layout](03-project-layout.md) | Where files go, and what MVT means here |

## Configuration

| | |
| --- | --- |
| [4. Settings](04-settings.md) | Declaring, validating, environments, `.env`, every setting |

## Handling requests

| | |
| --- | --- |
| [5. Routing](05-routing.md) | Patterns, methods, groups, mounting |
| [6. Named routes](06-named-routes.md) | Naming a route and building its URL |
| [7. Views and handlers](07-views.md) | Rendering, redirects, flash messages, JSON, generic views |
| [30. Forms and validation](30-forms.md) | Binding a request into a struct or a model |
| [31. File uploads](31-uploads.md) | Storage, validation, staging, serving |
| [32. Pagination](32-pagination.md) | Page sizes, controls, custom paginators |
| [33. Caching](33-caching.md) | Backends, Remember, fragments, page and query caching |
| [34. Internationalisation](34-internationalisation.md) | Catalogs, plurals, locale detection, RTL |
| [35. Email](35-email.md) | The `Sender` interface, four backends, your own backend |
| [36. Background jobs](36-jobs.md) | Declaring, enqueueing, workers, retries, scheduling |
| [8. Templates](08-templates.md) | Layouts, partials, pages, functions |
| [9. Static files](09-static-files.md) | Embedded or from disk |

## Data

| | |
| --- | --- |
| [10. Models](10-models.md) | Defining entities, tags, registration |
| [11. Database](11-database.md) | Connections, engines, querying with GORM |
| [12. Migrations](12-migrations.md) | Versioned schema changes and the ledger |

## Users

| | |
| --- | --- |
| [13. Sessions](13-sessions.md) | The session API and the three backends |
| [14. Authentication](14-authentication.md) | The user entity, passwords, sign-in, guards |
| [15. Admin portal](15-admin.md) | Mounting it, and CRUD for your own models |
| [28. Permissions and roles](28-permissions.md) | Four permissions per model, bundled into roles |
| [29. Self-service accounts](29-accounts.md) | Sign-in, registration and profile pages for your users |

## Middleware and security

| | |
| --- | --- |
| [16. Middleware](16-middleware.md) | What runs by default, and adding your own |
| [17. Security headers and CSP](17-security-headers.md) | Headers, allowed hosts, nonces |
| [18. CSRF](18-csrf.md) | Tokens in forms and in fetch |
| [19. Rate limiting](19-rate-limiting.md) | Token buckets, client keys |
| [20. CORS](20-cors.md) | Origins, preflights, what CORS is not |
| [21. Compression](21-compression.md) | When gzip is applied |
| [22. HTTPS and certificates](22-https.md) | TLS, Let's Encrypt, running behind a proxy |

## Operations

| | |
| --- | --- |
| [23. The CLI](23-cli.md) | Every command and flag |
| [24. First run](24-first-run.md) | Bootstrapping a fresh install |
| [25. Deployment](25-deployment.md) | Building, checklist, shutdown, logging |
| [26. Testing](26-testing.md) | Testing your app, and the framework's own suite |
| [27. Architecture](27-architecture.md) | Layering, seams, what is not extensible yet |
