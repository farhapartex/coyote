# Database

[← Back to contents](README.md)

Persistence uses GORM under the hood, with a cgo-free SQLite driver. Both arrive with the framework
— there is nothing extra to install and no driver import to remember.

## Configuration

You get a SQLite database without configuring anything:

```go
settings.Default().Databases
// []Database{{Alias: "default", Engine: "sqlite", Name: "<BaseDir>/coyote.db"}}
```

`Databases` is a list and **the first entry is the default connection**. Replace it to point
somewhere else, keeping the one you want as default first:

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
| `Engine` | `sqlite`, `postgres` or `mysql`. Blank defaults to `sqlite` |
| `Name` | SQLite file path, or the database name for a server engine |
| `Host` / `Port` | Server engines only; port defaults to 5432 or 3306 |
| `User` / `Password` | Server engines only; rejected on SQLite so mistakes are caught early |
| `Options` | Extra DSN parameters |
| `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime` | Pool tuning; zero means the driver default |

Relative SQLite paths resolve against `BaseDir`, so the file lands in the project folder. Absolute
paths and `:memory:` are left alone.

Accessors: `s.Database()` for the default, `s.DatabaseByAlias("cache")` to look one up, `db.DSN()`
to build the connection string, and `db.Redacted()` to mask the password for display.
`settings.SQLiteDatabase(alias, name)` is a shorthand constructor.

Validation catches an empty list, unknown engines, duplicate aliases, a missing SQLite path,
credentials set on SQLite, a server engine without a host, and negative pool values — at startup,
not at first query.

## Querying

`a.DB()` hands you the GORM handle for the default connection. The connection is opened on first
use and shared afterwards.

```go
handle, err := a.DB()
if err != nil {
	return err
}

var products []Product
handle.Where("is_published = ?", true).Order("name").Find(&products)
```

This guide does not teach GORM — its own documentation does that better, and its API is not part of
Coyote's surface. What matters here is that you reach it through `a.DB()` and then use it exactly as
you would in any Go program:

```go
handle.First(&product, "sku = ?", sku)
handle.Create(&Product{ID: id.MustNew(), Name: "Kettle", SKU: "KTL-1"})
handle.Model(&product).Update("stock", 12)
handle.Delete(&Product{}, "id = ?", productID)

handle.Transaction(func(tx *gorm.DB) error {
	if err := tx.Create(&order).Error; err != nil {
		return err
	}
	return tx.Model(&Product{}).Where("id = ?", productID).
		Update("stock", gorm.Expr("stock - ?", 1)).Error
})
```

Handlers get their context the usual way, and passing it down is worth the habit — a cancelled
request then stops its own query:

```go
handle.WithContext(r.Context()).Find(&products)
```

### The pool

```go
pool, err := a.Pool()   // *sql.DB, for stats or raw database/sql work
a.CloseDB()             // closes it; Run() already does this on shutdown
```

## Transactions

```go
err := a.Transaction(r.Context(), func(tx *app.Tx) error {
	if _, err := tx.Store().Insert(ctx, schema, record); err != nil {
		return err
	}
	tx.Keep(&product.Photo)
	return tx.DB().Create(&line).Error
})
```

Return an error and everything rolls back; return nil and it commits. A panic rolls back and then
carries on panicking. `tx.Store()` is the generic store scoped to the transaction, so it sees the
transaction's own uncommitted writes.

`tx.Keep(&ref)` is the piece worth knowing about: [uploaded files](31-uploads.md) named this way are
promoted out of staging **only after the commit succeeds**. If the transaction rolls back, the bytes
stay in staging and the sweeper reclaims them — so a failed write can never leave a file that nothing
references, and a successful one can never lose its file.

## Filtering, sorting and counting

The generic store takes a `model.Query`:

```go
page, err := store.List(ctx, schema, model.Query{
	Filters: []model.Filter{
		{Column: "is_published", Op: model.Eq, Value: true},
		{Column: "price", Op: model.Lte, Value: 50},
		{Column: "status", Op: model.In, Value: []any{"live", "draft"}},
	},
	Sort:   "-created_at",
	Select: []string{"id", "name", "price"},
	Limit:  20,
})
```

| Operator | Meaning |
| --- | --- |
| `Eq`, `Ne` | equal, not equal |
| `Lt`, `Lte`, `Gt`, `Gte` | comparisons |
| `In` | one of a `[]any`; an empty list matches nothing |
| `Like` | pattern match, your `%` placement |
| `Null`, `NotNull` | is or is not null |

Three safety properties, since these usually come from a URL:

- **Columns are checked against the schema.** An unknown column is `store.ErrBadFilter`, never SQL.
- **Values are always bind parameters**, and operators come from a closed set, so neither can be
  injected.
- **`Sort` is validated**, accepting `name`, `-name` or `name desc` and rejecting everything else. It
  is safe to pass `?sort=` straight in. `Order` remains a raw escape hatch for strings *you* write —
  never for user input.

`Select` narrows the columns fetched, which is worth doing on tables with a large text column; the
primary key is always included so rows stay addressable.

Alongside `List` there are `Count`, `Exists` and `First`, all sharing the same conditions — so a
filtered list's `Total` always agrees with its rows.

## Relations

A belongs-to is described automatically:

```go
type Item struct {
	ID         string  `gorm:"primaryKey;size:64"`
	Title      string  `gorm:"size:100;not null"`
	CategoryID *string `gorm:"size:64;index"`
	Category   Category
}
```

```go
schema.Relations   // [{Column: "category_id", Target: "categories", TargetKey: "id", LabelColumn: "name"}]
```

Ask for a relation and each record gains a readable label beside the key:

```go
page, _ := store.List(ctx, schema, model.Query{With: []string{"category_id"}})

page.Records[0].Get("category_id")            // "c1"
page.Records[0].Get("category_id__label")     // "Kitchen"
```

**The cost is one extra query per relation, not per row.** The distinct keys on the page are
collected and resolved with a single `IN (…)`, so a page of 20 rows costs two queries and a page of
200 still costs two. Nothing is resolved unless `With` asks, so a plain list stays a single query.

The label column is the target's `name`, `title`, `label`, `username` or `email`, whichever exists
first, falling back to its first string column.

Has-many and many-to-many are not described yet.

## Querying without GORM

The admin portal never touches GORM directly. It goes through `model.Store`, a five-method port
that any backend can satisfy:

```go
type Store interface {
	List(ctx context.Context, schema *Schema, query Query) (Page, error)
	Find(ctx context.Context, schema *Schema, id string) (Record, error)
	Insert(ctx context.Context, schema *Schema, record Record) (string, error)
	Update(ctx context.Context, schema *Schema, id string, record Record) error
	Delete(ctx context.Context, schema *Schema, id string) error
	Count(ctx context.Context, schema *Schema, query Query) (int64, error)
	Exists(ctx context.Context, schema *Schema, query Query) (bool, error)
	First(ctx context.Context, schema *Schema, query Query) (Record, error)
	WithTx(tx *gorm.DB) Store
}
```

```go
store, err := a.Store()
schema, err := a.Describe(Product{})

page, err := store.List(r.Context(), schema, model.Query{
	Limit: 20, Offset: 0, Sort: "-name",
	Filters: []model.Filter{{Column: "is_published", Op: model.Eq, Value: true}},
})
for _, record := range page.Records {
	fmt.Println(record.String("name"), record.Get("price"))
}
```

Use it when you want the same column-driven, schema-derived access the admin uses — generic code
over records. Use `a.DB()` when you want typed structs and the full query language. Both talk to the
same database.

Reads through this port can be cached per table, opt in with `store.Cached` — see
[Caching](33-caching.md).

## Multiple connections

`a.DB()` is the default connection. Reach another by alias, and it is opened once and reused:

```go
handle, err := a.DBByAlias("reports")
store, err := a.StoreForAlias("reports")
```

### Routing a model to a database

Declare where a model lives and the framework follows it:

```go
a.RegisterModel(
	model.Of(Product{}),                 // default connection
	model.On("reports", DailyTotal{}),   // the "reports" connection
)
```

```go
handle, err := a.DBFor(DailyTotal{})     // the reports connection
store, err := a.StoreFor(DailyTotal{})   // a store bound to it
schema, err := a.Describe(DailyTotal{})  // described against it
```

Every alias is closed on shutdown along with the default.

**One limit to know:** `makemigrations` and `migrate` run against the default connection only. A
routed model is reported rather than silently skipped —

```
not migrated: 1 model(s) are routed to another database ([reports]).
migrations run against the default connection only; create those tables yourself for now.
```

— so until per-alias migrations land, create those tables with your own migration or
`AutoMigrate` against that handle.

## Next

[Migrations →](12-migrations.md)
