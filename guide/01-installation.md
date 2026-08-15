# Installation

[← Back to contents](README.md)

You never clone the framework. It arrives as a Go module, and one command creates a project that
already builds.

## Requirements

| | |
| --- | --- |
| Go | 1.25 or newer |
| Database | nothing to install — SQLite works out of the box |
| C toolchain | not needed; the SQLite driver is cgo-free |

## 1. Install the command

```
go install github.com/farhapartex/coyote/cmd/coyote@latest
```

That is the only long path you will ever type. Everything after it is `coyote …`:

```
$ coyote version
coyote 0.4.0
```

If the shell cannot find it, `go install` put the binary somewhere not on your `PATH`:

```
export PATH="$PATH:$(go env GOPATH)/bin"
```

Add that line to your shell profile once.

## 2. Create a project

```
coyote new myshop
cd myshop
coyote start
```

`new` writes the project, then runs `go mod init`, pulls the framework, registers the management
command as a project tool, and tidies — so the directory it leaves behind compiles as it stands.

| Flag | Purpose |
| --- | --- |
| `--module=PATH` | module path for `go.mod`; defaults to the project name |
| `--force` | scaffold into a directory that is not empty |
| `--skip-deps` | write the files and skip the `go` commands |

```
coyote new myshop --module=github.com/jane/myshop
```

## What you get

```
myshop/
  go.mod
  main.go              routes and mounting
  settings.go          configuration
  migrations/          generated migrations; commit them
  templates/
    layouts/base.html
    pages/home.html
  static/site.css
  .env                 local values, including a generated SecretKey
  .env.example         the same keys, without the secret
  .gitignore
  README.md
```

The `SecretKey` in `.env` is generated per project with `crypto/rand`, and `.env` is git-ignored
from the first commit — so no placeholder secret ever reaches your repository.

## What comes with the module

You do **not** install GORM, a SQLite driver, or a session library separately:

| Dependency | Why it is there |
| --- | --- |
| `gorm.io/gorm` | the ORM behind models, the admin portal, and migrations |
| `github.com/glebarez/sqlite` | pure-Go SQLite, so `CGO_ENABLED=0` builds work |
| `gorm.io/driver/postgres`, `gorm.io/driver/mysql` | the other two supported engines |
| `golang.org/x/crypto` | ACME/Let's Encrypt certificates |

Routing, sessions, templates, authentication, and the admin portal are standard library only.

## Adding Coyote to a project you already have

Skip `new` and wire the two files yourself:

```
go get github.com/farhapartex/coyote
go get -tool github.com/farhapartex/coyote/cmd/coyote
```

Then write `settings.go` and `main.go` as shown in the [quick start](02-quickstart.md). The second
line is optional; it pins the command to that project so everyone runs the same version:

```
go tool coyote start
```

Both forms accept the same commands.

## Next

[Quick start →](02-quickstart.md)
