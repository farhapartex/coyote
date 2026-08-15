# Installation

[← Back to contents](README.md)

## Requirements

| | |
| --- | --- |
| Go | 1.25 or newer |
| Database | nothing to install — SQLite works out of the box |
| C toolchain | not needed; the SQLite driver is cgo-free |

## Add the framework

```
go get github.com/farhapartex/coyote
```

That single command brings everything with it. You do **not** install GORM, a SQLite driver, or a
session library separately — they arrive as dependencies of the framework and are wired up for you:

| Dependency | Why it is there |
| --- | --- |
| `gorm.io/gorm` | the ORM behind models, the admin portal, and migrations |
| `github.com/glebarez/sqlite` | pure-Go SQLite, so `CGO_ENABLED=0` builds work |
| `gorm.io/driver/postgres`, `gorm.io/driver/mysql` | the other two supported engines |
| `golang.org/x/crypto` | ACME/Let's Encrypt certificates |

Routing, sessions, templates, authentication, and the admin portal are standard library only.

## Add the management command

The CLI is installed as a **Go tool**, so its version is pinned in your `go.mod` next to everything
else and every developer on the project runs the same one:

```
go get -tool github.com/farhapartex/coyote/cmd/coyote
```

Check it:

```
$ go tool coyote version
coyote 0.3.0
```

Run it from the directory that holds your `main.go` and `settings.go`.

> `go coyote start` is not possible. The `go` command cannot be extended with new subcommands, so
> `go tool coyote …` is the closest supported form.

## Next

[Quick start →](02-quickstart.md)
