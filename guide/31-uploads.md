# File uploads

[← Back to contents](README.md)

Uploads are off until you turn them on:

```go
s.Uploads = settings.Uploads{
	Enabled: true,
	Dir:     "media",
	MaxSize: 10 << 20,
	Allowed: []string{"image/jpeg", "image/png", "application/pdf"},
}
```

Nothing is served over HTTP unless you also ask for that — see [Serving](#serving).

## Taking a file

```go
func upload(w http.ResponseWriter, r *http.Request) {
	file, err := a.Uploads.Accept(r, "photo")
	if err != nil {
		view.Error(r, "That file was not accepted.")
		view.Redirect(w, r, "/profile")
		return
	}

	product.Photo = file.Ref
	db.Save(&product)

	a.Uploads.Commit(r.Context(), &product.Photo)
}
```

`Accept` caps the body, sniffs the type, hashes the bytes, and writes them to staging. What comes
back describes the result:

```go
type File struct {
	Ref      Ref        // where it lives
	Name     string     // the original name, sanitised, for display only
	Type     string     // the sniffed content type
	Size     int64
	Digest   string     // sha-256
	Uploaded time.Time
}
```

## Where files land

Three levels, checked in order:

1. **The field's own tag** — `coyote:"path=products/photos"`
2. **`Uploads.Path`** in settings, for a project-wide default
3. **The framework default** — no prefix, straight under the media directory

```go
type Product struct {
	Photo upload.Ref `gorm:"size:200" coyote:"path=products/photos,accept=image/*"`
}
```

That stores at `products/photos/ab/cd/<digest>.png` inside `Uploads.Dir`, regardless of what
`Uploads.Path` says. Paths are cleaned before use, so `../../etc` cannot climb out of the media
root.

`accept=` is passed through to the file input as its `accept` attribute — a convenience for the
browser, not a security control; the real check is the sniffed type against `Uploads.Allowed`.

## Storing the reference on a model

```go
type Product struct {
	ID    string     `gorm:"primaryKey;size:64"`
	Photo upload.Ref `gorm:"size:128"`
}
```

`upload.Ref` is a string underneath, with `Value`/`Scan` implemented, so it persists like any other
column and `makemigrations` sees nothing unusual.

Because the type marks itself as a file, the admin **renders a file input for it automatically** —
with a link to the current file and a remove checkbox when one is already stored. Any string column
can opt in the same way with `coyote:"file"`.

## The two-phase commit

This is the part most frameworks leave to you, and it is why Django projects accumulate dead files.

```
Accept  →  staged/<path>/ab/cd/<digest>.png   nothing references it yet
Commit  →  <path>/ab/cd/<digest>.png          promoted once your write succeeded
Delete  →  gone, or trash/… if you set a window
```

`staged/` and `trash/` are the only reserved areas; committed files sit at the root of your media
directory, under whatever path the field asks for.

- **A failed insert leaves an orphan in `staged/`**, and `Sweep` deletes anything there older than
  `StageTTL` (24h). Orphans clean themselves up. This has nothing to do with how long a signed URL
  lives — that is `SignedURLTTL`.
- **`Commit` runs after your database write succeeds.** If the write fails, you simply never commit,
  and the sweeper takes care of it.
- **`Delete` is immediate by default** (`TrashTTL: 0`). Set a window and deletes move to `trash/`
  instead, so a mistake stays recoverable for that long.

```go
staged, trashed, err := a.Uploads.Sweep(ctx)
```

Run it from a periodic job. It never touches committed files.

## What is checked, and why

The filename and the `Content-Type` header both come from the client, so neither is trusted.

| Check | What it stops |
| --- | --- |
| `MaxBytesReader` before the body is read | a huge POST exhausting memory |
| `Server.MaxBodyBytes` capping every request | a body reaching any handler unbounded |
| Sniffing the first 512 bytes | a `.png` that is really a script |
| Sniffed type must match the declared one | a spoofed header |
| Extension derived from the **sniffed** type | traversal and double extensions |
| The key is the content hash, never the name | `../../etc/passwd`, null bytes, reserved names |
| `image.DecodeConfig` over the whole file, against `MaxPixels` | decompression bombs, and images that will not decode |
| Empty files rejected | zero-byte junk |

The original filename is kept only as `File.Name`, sanitised, for showing back to the user. It never
touches a path.

Because the key is a content hash, identical uploads are stored once and the bytes under a key never
change — which is what makes the far-future cache header safe.

## Serving

Off by default, on purpose: user content served from your own origin is how stored XSS happens.

```go
s.Uploads.Serve = true
s.Uploads.URL   = "/media/"
```

When it is on, the handler always sets `X-Content-Type-Options: nosniff`, sends
`Content-Disposition: attachment` for anything that is not an image or PDF, serves through
`http.ServeContent` so range requests and video seeking work, and refuses to serve anything in
`staged/` or `trash/`.

### Private files

```go
s.Uploads.Private = true
```

Every request then needs a signature derived from `SecretKey`:

```go
url := a.MediaURL(product.Photo)   // signed and time-limited when Private is on
s.Uploads.SignedURLTTL = 15 * time.Minute
```

An unsigned, expired, or tampered URL gets a 403 — and so does one signed for longer than
`SignedURLTTL`, so a link that leaks into a chat log, a referrer header or an access log stops
working in minutes rather than a day. Fifteen minutes is the default. It is a *link* lifetime, quite
separate from `StageTTL`, which is only about how long an unclaimed upload waits before the sweeper
takes it.

In templates:

```html
<img src="{{media .Photo}}" alt="">
```

## Somewhere other than local disk

```go
s.Uploads.Storage = myS3Storage{}
```

```go
type Storage interface {
	Save(ctx context.Context, key string, r io.Reader) (Stat, error)
	Open(ctx context.Context, key string) (io.ReadSeekCloser, error)
	Stat(ctx context.Context, key string) (Stat, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool
	Move(ctx context.Context, from, to string) error
	List(ctx context.Context, prefix string) ([]Stat, error)
}
```

Nothing above this interface knows where the bytes are. The filesystem implementation validates
every key and writes through a temporary file, so a crash mid-write cannot leave a half file under a
real name.

## Not here yet

Thumbnails and resizing. Validation reads image headers only; it never decodes pixels.

## Next

[Pagination →](32-pagination.md)
