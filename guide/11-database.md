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
}
```

```go
store, err := a.Store()
schema, err := a.Describe(Product{})

page, err := store.List(r.Context(), schema, model.Query{
	Limit: 20, Offset: 0, Order: "name asc", Search: "kettle",
})
for _, record := range page.Records {
	fmt.Println(record.String("name"), record.Get("price"))
}
```

Use it when you want the same column-driven, schema-derived access the admin uses — generic code
over records. Use `a.DB()` when you want typed structs and the full query language. Both talk to the
same database.

## Multiple connections

`a.DB()` is the default connection. For a second one, open it yourself from the settings entry:

```go
cfg, ok := a.Settings.DatabaseByAlias("cache")
handle, err := db.Open(cfg, db.Options{})
```

## Next

[Migrations →](12-migrations.md)
