# Admin portal

[← Back to contents](README.md)

A working back office, mounted in one line. It lives in `contrib/`, so if you never call `Mount` it
is never compiled into your binary.

## Mounting

```go
portal := admin.Mount(a)
```

No options — it reads `Admin.Prefix`, `Admin.SiteName` and `Admin.Tagline` from settings:

```go
s.Admin.Prefix   = "/admin"
s.Admin.SiteName = "Acme administration"
s.Admin.Tagline  = "internal tools"
```

Out of the box you get a login screen, a dashboard, user management (create, edit, search, delete,
password reset), and a list of active sessions with revoke.

Access requires `IsStaff`. Managing users, roles and sessions requires `IsSuperadmin` — staff who
reach those URLs get a 403, and the sidebar does not show them the links.

What staff can do with your own managed resources is decided by
[permissions](28-permissions.md).

Built-in safety rules: editing your own account locks the role and status fields, self-deletion is
refused, the last active superadmin cannot be removed, and changing a password revokes that user's
other sessions.

## CRUD for your own models

Register a resource and the portal grows a full section for it. You write no routes, no handlers,
and no templates — everything is derived from the entity.

```go
type productResource struct{}

func (productResource) Entity() any { return Product{} }

portal.MustManage(productResource{})
```

That produces, in plural form:

```
GET  /admin/products              paged list
GET  /admin/products/new          empty form
POST /admin/products/new          create
GET  /admin/products/{id}         detail form
POST /admin/products/{id}         update
POST /admin/products/{id}/delete  delete
```

The section also appears in the sidebar. Call `Manage` after `Mount` and before `Run`.

`Manage` returns an error rather than panicking for the cases you can get wrong — `ErrReservedSlug`
(the portal already owns `users`, `sessions`, `login`, `logout`), `ErrDuplicateSlug`, and
`ErrNotMountedYet`. `MustManage` panics instead, which is what you want at startup.

## What the form derives

One input per column, chosen by type:

| Go type | Input |
| --- | --- |
| `string` | `text`, or `password` for password-ish names, `email` for `email` |
| `int`, `uint` | `number` |
| `float64` | `number` with `step="any"` |
| `bool` | `checkbox` |
| `time.Time` | `datetime-local` |

A `NOT NULL` column with no default is **mandatory**; nullable columns are optional. `size` becomes
`maxlength` and is validated server-side too. The primary key is never an input — it is generated (a
UUID for string keys, left to the database for autoincrement keys) and rendered read-only on the
detail page. `created_at` and `updated_at` are framework-managed and never editable.

## Customising through interfaces

`Resource` is the only interface you must satisfy. Everything else is opt-in — implement the ones
you need and the framework detects them:

```go
type Resource   interface { Entity() any }

type Labelled   interface { Label() string }           // singular label
type Pluralised interface { PluralLabel() string }     // heading and sidebar
type Routed     interface { Slug() string }            // URL segment
type Listed     interface { ListColumns() []string }   // which columns in the list
type Concealed  interface { HiddenColumns() []string } // never render these
type Sorted     interface { DefaultOrder() string }    // list ordering
type Guarded    interface { ReadOnly() bool }          // browse only, no writes
```

```go
func (productResource) PluralLabel() string    { return "Catalogue" }
func (productResource) Slug() string           { return "catalogue" }
func (productResource) ListColumns() []string  { return []string{"name", "sku", "price"} }
func (productResource) HiddenColumns() []string{ return []string{"internal_note"} }
func (productResource) DefaultOrder() string   { return "created_at desc" }
func (productResource) ReadOnly() bool         { return true }
```

## Adding your own page

`Register` puts a link and a handler of your own in the portal, guarded like every other admin
route. The slug is derived from the name unless you set one, and the handler is mounted under
`/admin/s/<slug>`:

```go
portal.Register(admin.Section{
	Name:        "Reports",
	Description: "Weekly sales figures",
	Handler:     reportsHandler,
})
```

## Why routes are generated per resource

`net/http` rejects `GET /admin/{resource}/new` as ambiguous against `GET /admin/users/{id}` — both
match `/admin/users/new` and neither is more specific. So routes are registered per resource at
`Manage` time instead of behind a single wildcard.

## What it depends on

```
core/model     Schema, Field, Kind, Describe — introspection from struct tags
core/model     Store, Record, Query — the data-access contract
core/store     the GORM-backed implementation
contrib/admin  resource interfaces, registry, generic handlers, two templates
```

The admin depends only on `model.Store`, never on `core/store`, so a different backend is a matter
of supplying another implementation. One list template and one form template serve every resource.

## Next

[Middleware →](16-middleware.md)
