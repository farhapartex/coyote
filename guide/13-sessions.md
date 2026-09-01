# Sessions

[← Back to contents](README.md)

Coyote is session-first: authentication state lives in the session, and the session is available in
every handler and every template without setup.

## Using one

```go
sess := session.FromRequest(r)

sess.Set("cart", items)
sess.Get("cart")                 // any
sess.GetString("who")            // typed readers
sess.GetInt("visits")
sess.GetBool("subscribed")
sess.Pop("one_time_value")       // read and remove
sess.Delete("cart")
sess.Keys()
sess.Clear()                     // empty it, keep the session
sess.Destroy()                   // end it and expire the cookie
```

The cookie is written **lazily**, just before the response headers go out, so a session mutated
anywhere in the handler chain is still persisted correctly. A session that stays empty for the whole
request is never written at all — anonymous traffic costs no cookie and no row.

## Flash messages

```go
view.Success(r, "Saved.")
view.Redirect(w, r, "/notes")
```

They survive exactly one redirect and drain when read. See [Views](07-views.md).

## Where sessions live

```go
s.Sessions.Backend = settings.SessionsInMemory   // default
s.Sessions.Backend = settings.SessionsInDB
s.Sessions.Backend = settings.SessionsInCookie
```

| | server storage | survives restart | revocable | notes |
| --- | --- | --- | --- | --- |
| `memory` | a map | no | yes | fast, zero setup; a restart signs everyone out |
| `database` | `sessions` table | yes | yes | shared across instances |
| `cookie` | none | yes | **no** | nothing to store, nothing to look up |

### memory

The default. Sessions are held in a map, swept every `CleanupInterval`. Sessions are stored as live
pointers, so a mutation is visible immediately and nothing is serialised.

### database

```go
s.Sessions.Backend = settings.SessionsInDB
```

Sessions go to a `sessions` table, which is a registered model like any other — so
`makemigrations` generates it and `migrate` creates it. Expired rows are hidden from reads and swept
periodically.

One thing to weigh: `Sessions.Rolling` marks the session modified on every request, so rolling
expiry plus the database backend means a write per request. Leave `Rolling` off unless you need it.

### cookie

```go
s.Sessions.Backend = settings.SessionsInCookie
```

The whole session travels in the cookie: gob-encoded, then sealed with AES-256-GCM under a key
derived from `SecretKey` with HKDF-SHA256. It is **encrypted, not merely signed**, so the contents
are unreadable to the client, and any edit fails authentication and is discarded as if no cookie had
been sent. The expiry travels inside the sealed payload, so it cannot be extended by editing the
cookie either. There is no store, no table, and no lookup on the read path.

The price is that there is nowhere to revoke from:

- Signing out deletes the browser's copy, but a cookie captured beforehand stays valid until it
  expires. Keep `Sessions.Lifetime` short.
- Changing a password cannot invalidate sessions already issued.
- The admin's session list reports that it is unavailable.
- Cookies are capped at 4 KB. A session that seals larger than that is refused, logged, and not
  written — keep cookie sessions small.
- Rotating `SecretKey` signs everybody out.

This backend requires a `SecretKey` in **every** environment, not just deployed ones.

## Values are gob, not JSON

JSON would turn every number into a `float64`, so `GetInt` would silently return `0` and `Flashes()`
would break. Gob keeps Go types intact. Custom types need registering once:

```go
session.RegisterValue(MyType{})
```

## Cookie settings

```go
s.Sessions.CookieName = "coyote_session"
s.Sessions.Lifetime   = 12 * time.Hour
s.Sessions.Rolling    = false            // extend the deadline on every request
s.Sessions.Secure     = true             // HTTPS only — on by default outside development
s.Sessions.HTTPOnly   = true             // JavaScript cannot read it
s.Sessions.SameSite   = settings.SameSiteLax
s.Sessions.Path       = "/"
s.Sessions.Domain     = ""
```

## Your own backend

```go
type Store interface {
	Load(ctx context.Context, id string) (*Session, bool)
	Save(ctx context.Context, s *Session) error
	Delete(ctx context.Context, id string) error
}
```

```go
s.Sessions.Store = myRedisStore{}
```

Add `Count`, `All` and `DeleteByUserID` — the `ManageableStore` interface — if you want the admin's
session list and revoke to work against it:

```go
type ManageableStore interface {
	Store
	Count(ctx context.Context) (int, error)
	All(ctx context.Context) ([]*Session, error)
	DeleteByUserID(ctx context.Context, userID string) (int, error)
}
```

**The context on a read is the request's, so a client that disconnects cancels it.** The context on
a write is deliberately not: the session is written on the way out, and a browser closing the
connection mid-response must not lose the login it just performed. The write carries the request's
values without its cancellation, so your store still sees the request id and locale.

## Session fixation

`a.Auth.Login` rotates the session id, so a token captured before sign-in is worthless afterwards.
You get that for free; no call is needed.

## Next

[Authentication →](14-authentication.md)
