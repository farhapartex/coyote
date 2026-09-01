# CSRF

[← Back to contents](README.md)

**CSRF protection is on for every route, and you do not switch it on.** Unsafe methods (POST, PUT,
PATCH, DELETE) that arrive without a valid token are rejected before your handler runs. Safe methods
pass through untouched.

## What you have to do

Put the token in your forms. That is the whole job:

```html
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
```

Forget it and the form gets a 403 the first time anyone submits it, in development, which is the
point.

## Letting something through

A webhook receiver has no session and no token to give, so exempt it by path prefix:

```go
s.Security.CSRFExempt = []string{"/hooks/"}
```

Prefixes are matched against the request path and must start with `/`. Keep the list short and keep
it obvious — it is the one place in your settings that says "these paths are forgeable, and that is
fine".

Turning the guard off everywhere is a setting rather than an accident:

```go
s.Security.CSRF = false
```

Validation refuses that while `Environment` is `staging` or `production`. Reach for it only for a
service that has no browser clients at all.

`a.CSRF` still exists as the per-group middleware, for the case where the global guard is off and one
group still needs it:

```go
forms := a.Group("/account", a.CSRF)
```

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
attach. Such an API is what `Security.CSRFExempt` is for.

## Next

[Rate limiting →](19-rate-limiting.md)
