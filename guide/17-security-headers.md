# Security headers and CSP

[← Back to contents](README.md)

## Always on

Every response carries:

| Header | Value | Why |
| --- | --- | --- |
| `X-Content-Type-Options` | `nosniff` | stops the browser guessing a content type |
| `X-Frame-Options` | `DENY` | no framing, so no clickjacking |
| `Referrer-Policy` | `same-origin` | full URLs stay inside your origin |
| `Cross-Origin-Opener-Policy` | `same-origin` | a page you open cannot reach back into yours |
| `Cross-Origin-Resource-Policy` | `same-origin` | other origins cannot embed your responses |
| `Permissions-Policy` | camera, microphone and geolocation denied | no silent access to hardware |
| `X-Request-Id` | a generated id | correlates with the access log |

Two of them are yours to change:

```go
s.Security.FrameOptions      = "SAMEORIGIN"   // or "" to omit the header entirely
s.Security.PermissionsPolicy = "camera=(self)"
```

`FrameOptions` accepts `DENY`, `SAMEORIGIN`, or empty; anything else is a startup error, because
`ALLOW-FROM` was removed from browsers years ago and a policy nobody enforces is worse than none —
use `frame-ancestors` in a CSP for that.

**`Cross-Origin-Opener-Policy` breaks a popup that talks back to its opener**, which is how some
OAuth flows finish, and `Cross-Origin-Resource-Policy` stops another origin embedding your images.
If you serve assets or run a popup sign-in, set those two yourself in a wrapper.

## Allowed hosts

`AllowedHosts` is enforced on every request; an unlisted `Host` header gets a 400 before any
handler runs. That closes host-header injection, cache poisoning, and password-reset links pointing
at someone else's domain.

```go
s.AllowedHosts = []string{"example.com", ".corp.internal"}
```

- `"*"` allows any host — development only.
- A leading dot matches subdomains: `.corp.internal` covers `api.corp.internal`.
- With `Debug` on, the check relaxes so `localhost` and `127.0.0.1` work.

It is required outside development, and a missing list is a startup error rather than a silent
open door.

## Content Security Policy

Off by default, because a policy that breaks your pages is worse than none.

```go
s.Security.CSP = settings.DefaultCSP
s.Security.CSPReportOnly = true    // observe first, enforce later
```

`DefaultCSP` allows nothing but your own origin, forbids objects and framing, and carries no
`unsafe-inline`.

### Nonces

Inline styles and scripts are handled with a nonce rather than by weakening the policy. Write
`{nonce}` anywhere in the policy string and each response gets a fresh one:

```go
s.Security.CSP = "default-src 'self'; style-src 'self' 'nonce-{nonce}'"
```

```html
<style nonce="{{.Nonce}}">body { color: #00ADD8 }</style>
```

In a handler:

```go
nonce := middleware.NonceFrom(r.Context())
```

The admin portal's own inline stylesheet already carries the nonce, so the portal keeps working
under the strict default.

### Rolling one out

1. Set `CSPReportOnly = true` and watch the browser console.
2. Fix what it reports — usually inline blocks that need the nonce.
3. Turn report-only off.

## Next

[CSRF →](18-csrf.md)
