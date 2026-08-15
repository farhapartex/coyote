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

## Reading form input

Standard library, all the way down:

```go
if err := r.ParseForm(); err != nil {
	http.Error(w, "400 bad request", http.StatusBadRequest)
	return
}
note := r.PostForm.Get("note")
```

There is no form/validation layer yet — validate in the handler, and return
`RenderStatus(..., 422, ...)` with the messages in your data.

## Next

[Templates →](08-templates.md)
