# The CLI

[← Back to contents](README.md)

One management command. Install it once:

```
go install github.com/farhapartex/coyote/cmd/coyote@latest
```

Every command except `new` runs from the directory holding your `main.go` and `settings.go`.

A project created by `coyote new` also pins the command as a project tool, so `go tool coyote
<command>` works there and runs the version recorded in that project's `go.mod`. Both forms accept
the same commands; `coyote` is shorter, `go tool coyote` is pinned.

## Commands

| Command | What it does |
| --- | --- |
| `coyote new <name>` | Create a new project in a directory of that name |
| `coyote start` | Build and run the project, reporting migration and user state first |
| `coyote makemigrations` | Diff your models against the snapshot and write a migration file |
| `coyote migrate` | Apply migrations that have not been applied yet |
| `coyote sqlmigrate` | Print the SQL a pending migration would run, without applying it |
| `coyote syncpermissions` | Create the four permissions for every registered model |
| `coyote collectstatic` | Fingerprint static files and write a manifest |
| `coyote createsuperadmin` | Create a superadmin who can sign in to the admin portal |
| `coyote version` | Print the coyote version |
| `coyote help` | List the commands |

## Flags

| Flag | Command | Purpose |
| --- | --- | --- |
| `--module=PATH` | `new` | Module path for `go.mod`; defaults to the project name |
| `--force` | `new` | Scaffold into a directory that is not empty |
| `--skip-deps` | `new` | Write the files without running the `go` toolchain |
| `--port=N` | `start` | Listen on N instead of the port in `settings.go` |
| `--host=H` | `start` | Bind to H instead of the host in `settings.go` |
| `--name=NAME` | `makemigrations` | Name the migration instead of guessing one |
| `--username=U` | `createsuperadmin` | Skip the username prompt |
| `--email=E` | `createsuperadmin` | Skip the email prompt |
| `--password=P` | `createsuperadmin` | Skip the password prompt |

`createsuperadmin` also reads `COYOTE_SUPERADMIN_USERNAME`, `_EMAIL` and `_PASSWORD` — the safer
route in scripts, since the prompt echoes what you type.

## A new project

```
coyote new myshop
cd myshop
coyote makemigrations --name=initial
coyote migrate
coyote createsuperadmin
coyote start
```

## How it works

The tool builds and runs the main package in the current directory, so **your** `settings.go`
applies. It passes the subcommand through `COYOTE_COMMAND` and any overrides through `COYOTE_PORT`
and `COYOTE_HOST`. Your `main.go` needs nothing beyond `app.New()` and `Run()` — `Run` dispatches to
whichever command was asked for.

```go
func main() {
	a := app.New()
	// routes
	log.Fatal(a.Run())   // serves, or runs the requested command
}
```

`a.Serve()` skips the dispatch and always serves, if you would rather wire commands yourself.

`new` is the exception to all of this: it runs entirely inside the `coyote` binary, because there is
no project to hand it to yet.

> `go coyote start` is not possible: the `go` command cannot be extended with new subcommands.

## Next

[First run →](24-first-run.md)
