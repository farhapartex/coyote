# Quick start

[← Back to contents](README.md)

From nothing to a running site with an admin portal.

## 1. Create the project

```
coyote new myshop
cd myshop
```

If `coyote` is not on your PATH yet, see [Installation](01-installation.md).

## 2. Start it

```
coyote start
```

```
WARN  this database has no schema yet and no migrations are declared; run: coyote makemigrations && coyote migrate
INFO  coyote listening url=http://127.0.0.1:8000 environment=development
```

The home page is already there. The warning is telling the truth: there are no tables yet, so
nobody can sign in to the admin portal.

## 3. Create the schema and an account

Leave the server running and use a second terminal:

```
coyote makemigrations --name=initial
coyote migrate
coyote createsuperadmin
```

Now open `http://127.0.0.1:8000/admin/` and sign in.

> The commands talk to the database directly, so the server does not have to be running — the
> second terminal is only because the first one is busy serving. See [First run](24-first-run.md).

## 4. Add a model

Create `models.go`:

```go
package main

import "time"

type Note struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:200;not null"`
	Body      string `gorm:"size:2000"`
	Published bool   `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

Register it in `main.go`, and hand it to the admin portal:

```go
import "github.com/farhapartex/coyote/core/model"

type noteResource struct{}

func (noteResource) Entity() any { return Note{} }

func main() {
	a := app.New()

	a.RegisterModel(model.Of(Note{}))

	portal := admin.Mount(a)
	portal.MustManage(noteResource{})

	...
}
```

Then:

```
coyote makemigrations --name=add_note
coyote migrate
```

Restart the server and `/admin/notes` is there — list, create, edit, delete — with no routes,
handlers, or templates written by you.

## 5. Add a page of your own

```go
a.Get("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
	a.Render(w, r, "pages/note.html", app.Data{"Title": "Note", "ID": r.PathValue("id")})
}).Named("note.detail")
```

```html
<!-- templates/pages/note.html -->
{{define "content"}}
  <h1>{{.Title}}</h1>
  <p>id: {{.ID}}</p>
{{end}}
```

## Where things live

```
main.go        routes and mounting
settings.go    configuration; read by everything else
models.go      your entities
migrations/    generated; commit them
templates/     layouts, partials, pages
static/        css, images, javascript
.env           local values, git-ignored
```

## Next

- [Settings →](04-settings.md) — what you can configure and how
- [Models →](10-models.md) — field types, tags, registration
- [Admin portal →](15-admin.md) — customising those generated screens
