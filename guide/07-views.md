# Views and handlers

[← Back to contents](README.md)

A view in Coyote is an ordinary `http.HandlerFunc`. There is no special signature, no context
object to learn, and no interface to satisfy — which means any middleware or helper written for
`net/http` works unchanged.

```go
func show(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}
```

## Rendering a page

```go
a.Get("/", func(w http.ResponseWriter, r *http.Request) {
	a.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})
})
```

`app.Data` is a `map[string]any` with three conveniences:

```go
data := app.Data{"Title": "Home"}.
	Set("Items", items).
	SetDefault("Title", "Fallback").      // only if absent
	Merge(app.Data{"Total": 12})
```

To render with a status other than 200 — a 404 page, a form that failed validation:

```go
a.RenderStatus(w, r, http.StatusNotFound, "pages/404.html", app.Data{"Title": "Not found"})
a.RenderStatus(w, r, http.StatusUnprocessableEntity, "pages/form.html", app.Data{"Errors": errs})
```

## The page for an address you never routed

`RenderStatus` covers a handler that decides a record is missing. A URL that matches no route at
all never reaches a handler, so give the application its own page for that once, at startup:

```go
a.SetNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	a.RenderStatus(w, r, http.StatusNotFound, "pages/404.html", app.Data{"Title": "Not found"})
}))
```

Without it, an unrouted path falls through to the standard library's plain `404 page not found`.

Two things it deliberately does **not** touch:

- a **method mismatch** still answers 405, because asking for the wrong verb on a real route is not
  a missing page
- a handler that renders **its own** 404 keeps it; the fallback only runs when no route matched

## What every template receives

On top of your own data, each render is given:

| Key | Value |
| --- | --- |
| `.User` | the signed-in `*auth.User`, or nil |
| `.Session` | the current session |
| `.CSRFToken` | the token for this session |
| `.Flashes` | flash messages, drained on read |
| `.Nonce` | the per-response CSP nonce |
| `.Path` | the request path |
| `.Request` | the `*http.Request` |
| `.Debug` | the `Debug` setting |
| `.Version` | the framework version |

`a.Context(r, data)` builds that same map without rendering, for when you want it in JSON.

## Redirects

```go
view.Redirect(w, r, "/notes")             // 303 See Other
view.RedirectPermanent(w, r, "/new-url")  // 301
```

303 is the right default after a POST: it turns the follow-up into a GET, so a refresh does not
resubmit the form.

## Flash messages

Flashes survive exactly one redirect and drain when read, which is what makes the
post-redirect-get cycle feel finished:

```go
view.Success(r, "Saved.")
view.Redirect(w, r, "/notes")
```

```go
view.Flash(r, "warning", "Check your input.")   // any kind you like
view.Success(r, "…")
view.Error(r, "…")
view.Warning(r, "…")
view.Info(r, "…")
```

```html
{{range .Flashes}}
  <p class="flash flash-{{.Kind}}">{{.Message}}</p>
{{end}}
```

Flashes live in the session, so they follow the [session backend](13-sessions.md) you chose.

## Safe redirect targets

After a login, `?next=` decides where the visitor lands — and an unchecked `next` is an open
redirect straight to a phishing page. `SafeNext` accepts same-origin paths only and falls back
otherwise:

```go
target := view.SafeNext(r.URL.Query().Get("next"), "/dashboard")
view.Redirect(w, r, target)
```

## JSON

```go
view.JSON(w, http.StatusOK, payload)
view.JSONError(w, http.StatusBadRequest, "that will not do")
view.JSONProblems(w, http.StatusUnprocessableEntity, problems)
```

Responses carry `application/json` and `nosniff`. Reading is deliberately strict:

```go
var in Payload
if err := view.Decode(r, &in); err != nil {
	view.JSONError(w, http.StatusBadRequest, err.Error())
	return
}
```

`Decode` caps the body at 1 MB (`DecodeLimit` for another size), refuses a non-JSON content type,
**rejects unknown fields** so a client typo is an error rather than a silently ignored value, and
refuses two JSON values in one body.

`view.WantsJSON(r)` tells you whether a handler should answer with JSON or HTML.

## Generic views

For the ordinary list/detail/create/update/delete shapes, the framework can supply the handler.
Django does this with class-based views; Go has no classes, so it is an options struct plus hooks:

```go
schema, _ := a.Describe(Product{})
store, _ := a.Store()

opts := view.Options{
	Store: store, Schema: schema, Renderer: a,
	Template: "pages/products.html",
	PerPage:  20,
	Order:    "name asc",
	Redirect: "/products",
}

a.Get("/products", view.List(opts))
a.Get("/products/{id}", view.Detail(opts))
a.Any("/products/new", view.Create(opts))
a.Any("/products/{id}/edit", view.Update(opts))
a.Post("/products/{id}/delete", view.Delete(opts))
```

Each returns an ordinary `http.HandlerFunc`, so middleware and guards compose as usual.

| Hook | Purpose |
| --- | --- |
| `Allow func(*http.Request) bool` | refuse with 403 before anything runs |
| `Filter func(*http.Request, model.Query) model.Query` | scope the query — tenant, owner, status |
| `Data func(*http.Request, view.Data) view.Data` | add your own template data |
| `IDParam` | the path wildcard holding the id, if not `id` |

`List` reads `?page=` and `?sort=` from the URL. `sort` is [validated against the
schema](11-database.md), so passing it straight through is safe.

Templates receive `.Records`, `.Total`, `.Page` and `.Sort` for a list, `.Record` for a detail, and
`.Record`, `.IsNew` and `.Problems` for a form. Binding and validation run through
[`core/form`](30-forms.md), and paging through [`Pagination`](32-pagination.md).

## Reading form input

Standard library, all the way down:

```go
if err := r.ParseForm(); err != nil {
	http.Error(w, "400 bad request", http.StatusBadRequest)
	return
}
note := r.PostForm.Get("note")
```

For anything beyond a field or two, bind and validate with [`core/form`](30-forms.md) instead of
reading values by hand.

## Next

[Templates →](08-templates.md)
