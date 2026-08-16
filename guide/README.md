<p align="center">
  <img src="../assets/coyote.png" alt="Coyote" width="180">
</p>

<h1 align="center">Coyote documentation</h1>

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
