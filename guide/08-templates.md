# Templates

[← Back to contents](README.md)

Pages are `html/template`, rendered through a layout. Nothing is invented on top of the standard
library, so autoescaping and the usual syntax behave exactly as documented in Go.

## The three kinds of file

```
templates/
  layouts/base.html     the page shell
  partials/nav.html     reusable fragments
  pages/home.html       one file per page
```

A layout defines the shell and leaves a hole:

```html
{{define "base.html"}}
<!doctype html>
<html>
  <head><title>{{.Title}}</title></head>
  <body>
    {{template "nav.html" .}}
    {{block "content" .}}{{end}}
  </body>
</html>
{{end}}
```

A page fills it:

```html
{{define "content"}}
  <h1>{{.Title}}</h1>
{{end}}
```

A partial is just a named template:

```html
{{define "nav.html"}}<nav><a href="{{url "home"}}">Home</a></nav>{{end}}
```

Render by page path:

```go
a.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})
```

## Where templates come from

Embedded (recommended — the binary is then self-contained):

```go
//go:embed templates
var templateFS embed.FS

func init() {
	templates, _ := fs.Sub(templateFS, "templates")
	settings.Configure(func(s *settings.Settings) {
		s.Templates.FS = templates
	})
}
```

From disk, which is occasionally handy in development:

```go
s.Templates.Dir = "templates"
```

Set one or the other, never both.

## Layout and shared files

```go
s.Templates.Layout = "layouts/base.html"                        // default
s.Templates.Shared = []string{"layouts/*.html", "partials/*.html"}   // default
```

Everything matched by `Shared` is parsed into every page, so partials are available without
importing them per page.

Using a second layout is a matter of naming it in the page's own define block, or setting
`Templates.Layout` for the app and keeping alternatives as partials.

## Built-in functions

| Function | Purpose |
| --- | --- |
| `url` | reverse a [named route](06-named-routes.md) |
| `static` | resolve a [fingerprinted asset](09-static-files.md) |
| `media` | build a URL for an [uploaded file](31-uploads.md) |
| `fragment` | render a partial through the [cache](33-caching.md) |

Translation is a method on the render context rather than a function, because a template function
cannot know which request is rendering — see [Internationalisation](34-internationalisation.md):

```html
{{.Locale.T "Save changes"}}
{{.Locale.N "%d note" "%d notes" (len .Notes)}}
```

Add your own:

```go
s.Templates.Funcs = template.FuncMap{
	"money": func(cents int) string { return fmt.Sprintf("£%.2f", float64(cents)/100) },
	"upper": strings.ToUpper,
}
```

```html
<p>{{money .PriceCents}}</p>
```

## Reloading

When `Debug` is true, templates are re-parsed on every request — edit, refresh, done. In staging
and production they are parsed once and cached. That is driven by `AutoReloadTemplates()`, which
follows `Debug`.

## Escaping and the CSP nonce

`html/template` escapes by value context, so `{{.Comment}}` is safe in HTML, in an attribute, and
in a `<script>` block. If you are enforcing a [Content Security Policy](17-security-headers.md),
inline blocks carry the nonce:

```html
<style nonce="{{.Nonce}}">body { color: #00ADD8 }</style>
```

## Next

[Static files →](09-static-files.md)
