# Routing

[← Back to contents](README.md)

Patterns go straight to `net/http.ServeMux`, so Go 1.22 method and wildcard syntax works as-is and
there is nothing new to learn about pattern matching.

## Methods

```go
a.Get("/items/{id}", show)
a.Post("/items", create)
a.Put("/items/{id}", replace)
a.Patch("/items/{id}", update)
a.Delete("/items/{id}", destroy)
a.Any("/webhook", hook)
```

Wildcards are read with the standard library:

```go
func show(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	...
}
```

## The trailing-slash rule

A `net/http` pattern ending in `/` matches a whole subtree. `Get("/", …)` therefore answers *every*
unmatched path and nothing ever 404s. Use `{$}` when you mean the exact path:

```go
a.Get("/{$}", home)      // only "/"
a.Get("/docs/", docs)    // "/docs/" and everything under it
```

## Groups

A group shares a prefix and, optionally, middleware:

```go
api := a.Group("/api", requireToken)
api.Get("/status", status)      // /api/status
api.Get("/items", listItems)    // /api/items
```

Groups nest, and prefixes accumulate.

## Middleware placement

Three scopes, from widest to narrowest:

```go
a.Use(middleware.RequireHTTPS)          // every request
api := a.Group("/api", requireToken)    // every route in the group
a.Post("/notes", createNote, a.CSRF)    // this route only
```

See [Middleware](16-middleware.md) for what runs before your handler by default.

## Mounting another handler

```go
a.Mount("/legacy", someOtherHandler)
```

Anything satisfying `http.Handler` works — a third-party router, a metrics endpoint, a gRPC-web
gateway. The prefix is stripped from the request before it reaches the mounted handler.

## Static files

```go
a.Static("/assets/", assetsFS)
```

Usually unnecessary — set `Static.FS` in settings instead and it is mounted for you. See
[Static files](09-static-files.md).

## Inspecting the table

```go
for _, route := range a.Routes() {
	log.Println(route.Method, route.Pattern, route.Name)
}
```

## Using the router alone

`router.New()` gives the same dispatch with none of the rest of the framework, if that is all you
want from Coyote.

## Next

[Named routes and reverse URLs →](06-named-routes.md)
