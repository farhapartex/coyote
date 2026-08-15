# Static files

[← Back to contents](README.md)

Static files are served only when you configure them. Nothing is exposed by default.

## From settings

```go
//go:embed static
var staticFS embed.FS

func init() {
	assets, _ := fs.Sub(staticFS, "static")

	settings.Configure(func(s *settings.Settings) {
		s.Static.URL = "/static/"
		s.Static.FS = assets
	})
}
```

```html
<link rel="stylesheet" href="/static/site.css">
```

Or from a directory, when you would rather not embed:

```go
s.Static.Dir = "static"
```

Set one or the other. `Static.URL` defaults to `/static/` and needs its trailing slash — it names a
subtree, not a single file.

## Ad hoc

Any other `fs.FS` can be mounted directly on a second prefix:

```go
a.Static("/downloads/", os.DirFS("/var/lib/myapp/files"))
```

## In production

Embedding keeps deployment to a single binary and is the usual choice. If you put a CDN or nginx in
front, leave `Static.FS` unset and let the proxy serve the directory — the framework will not
register the route at all, so there is no accidental second path to the same files.

## Next

[Models →](10-models.md)
