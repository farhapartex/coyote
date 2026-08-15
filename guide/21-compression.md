# Compression

[← Back to contents](README.md)

```go
s.Security.Compress = true
s.Security.CompressLevel = 6   // 1-9, or 0 for the default
```

Gzip, applied only where it helps. Responses are left alone:

- when the client did not ask for it,
- when the body is under 1 KB,
- when something already set `Content-Encoding`,
- when the status carries no body,
- and for content types that do not shrink — images, video, audio, zip, PDF.

`Vary: Accept-Encoding` is always set, **including on uncompressed responses**, so a cache cannot
hand a gzipped body to a client that cannot read it. `Content-Length` is dropped once the body is
compressed, and `Flush` still reaches the client, so streaming responses keep working.

If a CDN or reverse proxy already compresses, leave this off rather than doing the work twice.

## Next

[HTTPS and certificates →](22-https.md)
