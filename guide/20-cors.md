# CORS

[← Back to contents](README.md)

Off until you list origins. A request with no `Origin` header is left alone entirely.

```go
s.Security.CORS = settings.CORS{
	Origins:          []string{"https://app.example.com"},
	AllowCredentials: true,
	MaxAge:           10 * time.Minute,
}
```

| Field | Default |
| --- | --- |
| `Origins` | none — the switch that turns CORS on |
| `Methods` | the usual set |
| `Headers` | the usual set |
| `ExposeHeaders` | none |
| `AllowCredentials` | `false` |
| `MaxAge` | none |

Preflights are answered with 204, and `Vary` is set on `Origin` (plus the request-method and
request-header names on preflights) so caches cannot serve one origin's response to another.

## Two rules enforced at startup

Rather than discovered in a browser console:

```
Security.CORS cannot combine the "*" origin with AllowCredentials; browsers reject that pairing, so list the origins you mean
Security.CORS origin "app.example.com" needs a scheme, for example https://app.example.com
```

## What CORS is not

An unlisted origin still receives its response for a simple `GET` — the **browser** enforces the
block, not the server. Preflights are refused outright. If you need the server itself to reject the
request, that is authentication's job, not CORS's.

## Next

[Compression →](21-compression.md)
