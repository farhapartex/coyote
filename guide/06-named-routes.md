# Named routes and reverse URLs

[← Back to contents](README.md)

Hard-coded paths rot. Name a route once and build its URL from the name, so moving it is one edit
instead of a search.

## Naming

Every registration returns the route, so a name is one chained call:

```go
a.Get("/{$}", home).Named("home")
a.Get("/posts/{id}", show).Named("post.detail")
a.Get("/files/{path...}", serve).Named("file")
```

Naming is optional. Do it for routes whose URL you build elsewhere.

## From Go

```go
url, err := a.Reverse("post.detail", 42)   // "/posts/42"
url := a.MustReverse("post.detail", 42)    // panics instead of returning an error
```

## From a template

The `url` function is built in:

```html
<a href="{{url "post.detail" .ID}}">read</a>
<a href="{{url "home"}}">home</a>
```

## Rules

- Values fill the wildcards **in order** and are path-escaped.
- `{path...}` keeps its slashes; everything else is escaped.
- Group prefixes are included — `/status/{code}` inside `a.Group("/api")` reverses to
  `/api/status/200`.
- A wrong name, a missing value, an empty value, or too many values is an error. In a template that
  fails the render rather than emitting a broken link.
- Naming two routes the same panics at registration, where you see it immediately.

```go
a.Reverse("nope")                  // error: unknown name
a.Reverse("post.detail")           // error: wrong arity
a.Reverse("post.detail", 1, 2)     // error: wrong arity
```

## Next

[Views and handlers →](07-views.md)
