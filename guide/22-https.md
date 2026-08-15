# HTTPS and certificates

[← Back to contents](README.md)

## With your own certificate

```go
s.Server.TLS.CertFile = settings.Env("TLS_CERT", "")
s.Server.TLS.KeyFile  = settings.Env("TLS_KEY", "")
s.Server.TLS.HSTS     = 30 * 24 * time.Hour
```

Both files set means `Run` serves over TLS:

```
INFO coyote listening url=https://127.0.0.1:8443 environment=production tls=true
```

**HTTP/2 comes with it.** Go negotiates h2 over TLS automatically — no setting, no dependency.

`MinVersion` defaults to TLS 1.2. For anything the framework does not model there is
`Server.TLS.Config`, a `*tls.Config` used as-is (mTLS, a custom cipher list, your own ACME manager),
and `Server.Configure func(*http.Server)`, called just before listening — that is where h2c goes if
you need cleartext HTTP/2 behind a proxy.

## From Let's Encrypt

Coyote can obtain and renew certificates itself, so a bare VM needs no reverse proxy:

```go
s.Server.TLS.Autocert  = true
s.Server.TLS.AcceptTOS = true
s.Server.TLS.Staging   = settings.EnvBool("ACME_STAGING", false)
```

**Hosts come from `AllowedHosts`** — the names you already declare are the names certificates are
issued for, so there is no second list to keep in step. Certificates are cached in
`Server.TLS.CacheDir` (`certs/` under `BaseDir` by default); keep that directory across deploys or
you will re-issue on every restart and meet the rate limits.

Three things worth knowing:

- **`AcceptTOS` is deliberately explicit.** Issuing a certificate means agreeing to the authority's
  terms, and the framework will not do that for you behind a default.
- **Validation happens over TLS on port 443** (TLS-ALPN-01), so no second listener on :80 is needed
  — but the port must be 443 and reachable from the internet.
- **Use `Staging` while you iterate.** Let's Encrypt's production rate limits are strict and a deploy
  loop will hit them; the staging directory issues untrusted certificates without the limits.

Validation refuses the combinations that would otherwise fail at runtime, in the dark:

```
coyote/settings: improperly configured:
  - Server.TLS.Autocert needs Server.TLS.AcceptTOS set to true; issuing a certificate means agreeing to the certificate authority's terms of service
  - Server.TLS.Autocert cannot use the "*" host; a certificate authority needs real host names
  - Server.TLS.Autocert validates over TLS on port 443, so Server.Port must be 443; for anything else supply your own Server.TLS.Config
```

## Redirecting and pinning

```go
a.Use(middleware.RequireHTTPS)   // plain HTTP redirects to https
```

`middleware.HSTS` is added for you when `Server.TLS.HSTS` is set, and only emits the header on
connections that are actually secure. Setting HSTS without TLS is a configuration error — a browser
that sees it will refuse plain HTTP to your host for the whole duration.

Both are aware of `X-Forwarded-Proto`, so they work behind a terminating proxy.

## Behind a proxy

If nginx, Caddy, or a load balancer terminates TLS, leave `Server.TLS` unset and serve plain HTTP on
a private address. Keep `RequireHTTPS` — it reads the forwarded header — and set
`Sessions.Secure = true` so cookies never travel unencrypted.

## Next

[The CLI →](23-cli.md)
