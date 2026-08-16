# Project layout

[← Back to contents](README.md)

## Your project

Coyote does not impose a directory structure. The only hard rule is that `settings.go` sits in the
main package, beside `main.go`, so `init()` runs before anything reads configuration.

A small site stays flat:

```
myapp/
  main.go
  settings.go
  models.go
  handlers.go
  migrations/        generated; commit these
  templates/
    layouts/base.html
    partials/nav.html
    pages/home.html
  static/
  myapp.db
```

A larger one moves the work into packages and keeps the main package as wiring:

```
myapp/
  main.go            routes and mounting
  settings.go
  internal/
    catalog/         entities, handlers, resources for one area
    billing/
  migrations/
  templates/
  static/
```

Handlers are plain `http.HandlerFunc`, so nothing about your own packages needs to know the
framework exists beyond the arguments you pass them.

## MVT

Coyote follows Model–View–Template, the same split Django uses. The names differ from MVC:

| Layer | What it is | Where |
| --- | --- | --- |
| **Model** | entities and data access | your structs, `core/model`, `core/store` |
| **View** | the code that answers a request | your handlers, `core/view` |
| **Template** | the HTML | your `templates/`, `core/template` |

Routing plays the part of Django's `urls.py` and lives in `core/router`.

## The framework itself

Useful when you are reading the source or deciding what to import.

```
lib/            depends on nothing local
  text/         casing, slugs, folding, truncation
  dotenv/       the .env parser
  id/           UUID generation
core/           what the framework needs to exist
  app/          the application object: wiring, lifecycle, render entry point
  router/       URL dispatch, groups, static, route table
  view/         template data, redirects, flash messages
  template/     html/template with layouts and partials
  model/        entity metadata, registry, the Store port
  store/        the GORM adapters behind model.Store and auth.Store
  auth/         the User entity, passwords, login guards
  db/           connections, pool tuning, pragmas
  session/      Session, stores, cookie manager, sealing
  middleware/   logging, recovery, hosts, headers, CSRF, CSP, CORS, limits
  settings/     the settings type, defaults, validation, env helpers
contrib/        what you opt into
  admin/        the admin portal
  cli/          management commands
  migrate/      migration operations, diffing, the ledger
  scaffold/     the project template behind `coyote new`
cmd/coyote/     the `coyote` command
```

`core` never imports `contrib`. That is why the admin portal lives in `contrib/admin`: call
`admin.Mount(a)` or it is never compiled into your binary.

For the dependency rules and the tests that enforce them, see [Architecture](27-architecture.md).

## Next

[Settings →](04-settings.md)
