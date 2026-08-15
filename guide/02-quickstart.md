# Quick start

[← Back to contents](README.md)

A working site with an admin portal, from an empty directory. Nothing here is skipped — this is the
whole file list.

## 1. Create the module

```
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/farhapartex/coyote
go get -tool github.com/farhapartex/coyote/cmd/coyote
```

## 2. `settings.go`

Every Coyote project declares its settings in code, and the app refuses to start without them.

```go
package main

import (
	"embed"
	"io/fs"

	"github.com/farhapartex/coyote/core/settings"
)

//go:embed templates
var templateFS embed.FS

func init() {
	templates, _ := fs.Sub(templateFS, "templates")

	settings.Configure(func(s *settings.Settings) {
		s.Debug = true
		s.SecretKey = settings.Env("SECRET_KEY", "development-only-key-change-me-please")
		s.AllowedHosts = []string{"127.0.0.1", "localhost"}
		s.Server.Port = 8000
		s.Templates.FS = templates
		s.Admin.SiteName = "Hello admin"
	})
}
```

`init()` is deliberate: it runs before `main`, so settings exist by the time anything reads them.

## 3. `main.go`

```go
package main

import (
	"log"
	"net/http"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
)

func main() {
	a := app.New()

	admin.Mount(a)

	a.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})
	}).Named("home")

	log.Fatal(a.Run())
}
```

## 4. Templates

```
templates/
  layouts/base.html
  pages/home.html
```

```html
<!-- templates/layouts/base.html -->
{{define "base.html"}}
<!doctype html>
<html>
  <head><title>{{.Title}}</title></head>
  <body>{{block "content" .}}{{end}}</body>
</html>
{{end}}
```

```html
<!-- templates/pages/home.html -->
{{define "content"}}
  <h1>Hello from Coyote</h1>
  {{if .User}}<p>Signed in as {{.User.DisplayName}}</p>{{end}}
{{end}}
```

## 5. Create the schema and an account

A fresh project has no tables and no users, so nobody could sign in to the portal yet. Three
commands fix that, once:

```
go tool coyote makemigrations --name=initial
go tool coyote migrate
go tool coyote createsuperadmin
```

`migrate` needs the server running, so start it in another terminal first — see
[First run](24-first-run.md) for the full sequence and the warnings the framework prints when you
forget a step.

## 6. Run it

```
go tool coyote start
```

```
INFO coyote listening url=http://127.0.0.1:8000 environment=development
```

Open `http://localhost:8000/` and `http://localhost:8000/admin/`.

## What you have now

```
hello/
  go.mod
  main.go
  settings.go
  migrations/
  templates/
  coyote.db
```

## Next

- [Project layout →](03-project-layout.md) — where to put things as this grows
- [Routing →](05-routing.md) — more than one page
- [Models →](10-models.md) — your own tables, and CRUD screens for them
