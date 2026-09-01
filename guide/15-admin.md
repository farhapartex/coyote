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

## Search, filters and sorting

**Column headers are sortable already** — clicking one sorts ascending, clicking again descending.
The value goes through the schema, so a hand-edited `?sort=` that names anything other than a real
column is ignored rather than executed.

Search and filters are **opt-in per resource**, because a search box that scans every column of a
wide table is a performance trap you did not ask for:

```go
func (productResource) SearchColumns() []string { return []string{"name", "sku"} }
func (productResource) FilterColumns() []string { return []string{"is_published"} }
```

- `SearchColumns` renders a search box and matches the term against those columns with `LIKE`, joined
  by OR. `%` and `_` in the term are escaped, so searching for `100%` looks for a literal `100%`.
- `FilterColumns` renders an any/yes/no dropdown per column — useful for the boolean flags most
  models have.
- Declare neither and the list stays exactly as it was: no box, no dropdowns, one query.

Search, filters, sort and page all compose, and the row count in the heading reflects the filtered
total rather than the whole table.

## Bulk actions

Select rows with the checkboxes and apply an action. **Delete** is built in and appears only for
someone who holds the resource's delete permission.

Your own actions come from an interface:

```go
func (productResource) Actions() []admin.Action {
	return []admin.Action{{
		Name:       "publish",
		Label:      "Publish",
		Permission: auth.ActionUpdate,
		Run: func(r *http.Request, ids []string) (int, error) {
			return publish(r.Context(), ids)
		},
	}}
}
```

`Run` receives the selected ids and returns how many it changed. An error is shown to the operator
verbatim, so write it for them. Nothing is deleted or changed unless an action was chosen and rows
were ticked.

**Every action is behind a permission.** `Permission` names one of
[the four actions](28-permissions.md) on this resource, and left empty it means `update` — so an
action you write without thinking about it is guarded rather than open. Reaching the bulk route at
all needs `read`, on the principle that you cannot act on rows you are not allowed to see. Naming a
permission outside the standard four works, but nothing generates those rows for you, so only a
superadmin would ever hold one.

One action takes at most 1000 rows. Over that it is refused rather than truncated, because an action
that silently half-ran is worse than one that did not run.

## Relation fields

A belongs-to column renders as a `<select>` of the target rows, labelled by the target's `name`,
`title`, `label`, `username` or `email` — whichever it has. Nullable keys get an empty choice.

Lists show the **label** rather than the raw key, resolved in one extra query per relation rather
than one per row. See [Relations](11-database.md).

Options are capped at 200 rows; beyond that a select is the wrong control and you want a search
field, which is not built yet.

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

## In other languages

Every string in the portal goes through the framework's translation layer, and a French catalog ships
with it. Add `fr` to `I18N.Supported` and the portal is in French; supply your own catalog entry for a
message id to override one of ours. The layout is right-to-left correct for Arabic, Hebrew and the rest.
See [Internationalisation](34-internationalisation.md).

## Next

[Middleware →](16-middleware.md)
