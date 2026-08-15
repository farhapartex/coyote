# CSRF

[← Back to contents](README.md)

`a.CSRF` rejects unsafe methods (POST, PUT, PATCH, DELETE) that arrive without a valid token. Safe
methods pass through untouched.

## Applying it

Per route, which is the usual case:

```go
a.Post("/notes", createNote, a.CSRF)
```

Per group:

```go
forms := a.Group("/account", a.CSRF)
```

Everywhere:

```go
a.Use(a.CSRF)
```

The admin portal applies it to all of its own routes already.

## The token

It is per-session and compared in constant time. Templates get it as `.CSRFToken`:

```html
<form method="post" action="/notes">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  <textarea name="note"></textarea>
  <button>Save</button>
</form>
```

For fetch/XHR, send it as a header instead:

```js
fetch("/notes", {
  method: "POST",
  headers: {"X-CSRF-Token": token},
  body: data,
})
```

The middleware reads the `csrf_token` form field first, then the `X-CSRF-Token` header.

## Rejection

A missing or wrong token is a `403 CSRF token invalid or missing`. Nothing reaches your handler.

## Why the session is involved

The token is derived per session, so it changes when the session does — including on login, where
the session id rotates. That is also why an API authenticated by a bearer token rather than a
session cookie does not need CSRF protection: there is no ambient credential for a browser to
attach.

## Next

[Rate limiting →](19-rate-limiting.md)
