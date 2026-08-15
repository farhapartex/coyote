# Testing

[← Back to contents](README.md)

## Testing your own app

An app built with `app.NewFrom` takes settings directly instead of the global ones, so a test can
build a throwaway instance and drive it with `httptest` — no server, no ports, no fixtures.

```go
func testApp(t *testing.T) *app.App {
	t.Helper()

	s, err := settings.New(func(s *settings.Settings) {
		s.Debug = true
		s.SecretKey = "test-secret-key-that-is-long-enough"
		s.AllowedHosts = []string{"*"}
		s.Templates.FS = templates
		s.Auth.PBKDF2Iterations = 1000            // keep hashing fast in tests
		s.Databases = []settings.Database{{
			Engine: settings.SQLite,
			Name:   filepath.Join(t.TempDir(), "test.db"),
		}}
	})
	if err != nil {
		t.Fatal(err)
	}
	return app.NewFrom(s)
}
```

```go
func TestHomePage(t *testing.T) {
	a := testApp(t)
	a.Get("/{$}", home)

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
```

Two things worth knowing:

- Use `a.Handler()`, not `a.Router` — the handler is what carries sessions, auth, and the security
  middleware.
- Lower `Auth.PBKDF2Iterations`. At the production 600,000 a test that creates a few users spends
  most of its time hashing.

### A schema for the test database

```go
migrate.Sync(handle, a.Models())
```

`Sync` runs `AutoMigrate` with no files and no ledger — exactly what a temporary database wants.

### Signing a request in

Drive the login route and keep the cookie, the same way a browser does:

```go
rec := httptest.NewRecorder()
a.Handler().ServeHTTP(rec, loginRequest)

next := httptest.NewRequest(http.MethodGet, "/account", nil)
for _, c := range rec.Result().Cookies() {
	next.AddCookie(c)
}
```

Remember CSRF on POSTs: read `{{.CSRFToken}}` out of the form page first, or apply `a.CSRF` only
where you mean it.

## The framework's own suite

```
go test ./tests/
go test ./tests/ -run Admin -v
go test -race ./tests/
```

It lives in one root package, `tests`, and touches only exported API — so a change that breaks a
caller breaks a test. One file per package under test, plus shared builders in `helpers_test.go`.

> The `_test.go` suffix is required: `go test` collects `TestXxx` only from files that carry it.
> Drop the suffix and those tests stop running while `go test` still reports `ok` — a silent hole,
> not an error.

## Next

[Architecture →](27-architecture.md)
