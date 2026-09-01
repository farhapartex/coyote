# Models

[← Back to contents](README.md)

A model is an ordinary Go struct. Coyote reads its shape to build the schema, the migrations, and
the admin screens — you never describe a table twice.

## Defining one

```go
type Product struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:200;not null"`
	SKU         string `gorm:"uniqueIndex;size:64;not null"`
	PriceCents  int64
	Stock       int
	Description string `gorm:"size:2000"`
	IsPublished bool   `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

Column names are derived from field names — `IsPublished` becomes `is_published` — so tags appear
only where behaviour is needed.

| Tag | Effect |
| --- | --- |
| `primaryKey` | the identifier; string keys get a generated UUID |
| `size:200` | column length, and `maxlength` on the admin form |
| `not null` | required, and mandatory on the admin form |
| `index` | plain index |
| `uniqueIndex` | unique index |
| `default:true` | column default |
| `column:name` | override the derived column name |
| `-` | ignore the field entirely |

A separate `coyote` tag carries upload settings for a file column:

```go
Photo upload.Ref `gorm:"size:200" coyote:"path=products/photos,accept=image/*"`
Manual string    `gorm:"size:200" coyote:"file,path=manuals"`
```

`file` marks a plain string column as a file, `path=` chooses where it is stored, and `accept=`
reaches the browser's file picker. See [File uploads](31-uploads.md).

`CreatedAt` and `UpdatedAt` are managed for you when present. A pointer field (`*time.Time`) is
nullable; a value field is not.

## Field types

| Go type | Kind | Admin input |
| --- | --- | --- |
| `string` | `string` / `text` | `text` (`password`, `email` by name) |
| `int`, `uint`, and their sized forms | `int` | `number` |
| `float32`, `float64` | `float` | `number` with `step="any"` |
| `bool` | `bool` | `checkbox` |
| `time.Time` | `time` | `datetime-local` |
| `[]byte` | `bytes` | — |
| `upload.Ref` | `file` | `file`, with a link to the current file |

## Defaults reach the DDL verbatim

A `default:` in a gorm tag is written into the generated `CREATE TABLE` exactly as you typed it, so
a string default needs its own quotes:

```go
Status string `gorm:"size:20;not null;default:'draft'"`
```

`default:draft` without them produces `DEFAULT draft`, which the database reads as a column name.
The value is SQL, and it is yours.

## Sensitive fields

A sensitive field is rendered as a password input, left alone when its form field comes back empty,
and kept out of admin list columns. Say so and it is settled:

```go
RecoveryPin string `gorm:"size:20" coyote:"sensitive"`
```

Without a tag the column name is used as a guess: `password`, `secret`, `token`, `api_key`, `salt`
and `pin` exactly, anything ending `_password`, `_secret`, `_token`, `_hash` or `_key`, and anything
starting `password` or `otp`. That guess is deliberately generous, because a leaked column is worse
than a hidden one — so `sort_key` is treated as sensitive too. Turn it off where the guess is wrong:

```go
CacheKey string `gorm:"size:100" coyote:"public"`
```

There is no decimal kind. Store money as an integer count of minor units — cents, pence, paise — and
format it on the way out, as the `PriceCents` field above does:

```go
PriceCents int64
```

`float64` is the wrong shape for currency. Most decimal prices have no exact binary form — `19.99` is
held as `19.98999999999999844` — and the error compounds as you add: `0.1` summed ten times is not
`1.0`, so a total and a `SUM` over the same rows can disagree in the last cent. An `int64` of minor
units is exact, sorts and sums correctly in every supported database, and needs one division to
display — see the `money` template function in [Templates](08-templates.md).

Use `float` for quantities that really are approximate: weights, distances, ratings.

## Registering it

Registration is what makes a model visible to `makemigrations`, to the admin portal, and to schema
introspection:

```go
func main() {
	a := app.New()

	a.RegisterModel(model.Of(Product{}), model.Of(Checkout{}))
	...
}
```

Use `model.Named` when the table name should not be derived:

```go
a.RegisterModel(model.Named("catalogue_products", Product{}))
```

`a.Models()` returns everything registered, including the framework's own (`users`, and `sessions`
when the [database session backend](13-sessions.md) is on).

After registering a new model, generate and apply its migration:

```
coyote makemigrations --name=add_product
coyote migrate
```

## Reading the derived schema

Occasionally useful in your own code — it is what the admin portal builds forms from:

```go
schema, err := a.Describe(Product{})

schema.Columns()          // every column name
schema.Visible()          // fields not hidden
schema.FormFields()       // fields that belong on a form
schema.ListFields()       // fields for a list view
schema.Field("sku")       // one field, with its Kind, Size, Nullable, Required
```

## Identifiers

String primary keys are filled with a UUID at insert time, so an id exists before the row does and
is stable across databases. Integer keys are left to the database's autoincrement. Either way the
key is never an editable input in the admin — it is generated, then shown read-only.

## Next

- [Database →](11-database.md) — connecting, and querying with GORM
- [Migrations →](12-migrations.md) — turning a struct change into a schema change
- [Admin portal →](15-admin.md) — CRUD screens for the model you just wrote
