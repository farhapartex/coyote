# The CLI

[← Back to contents](README.md)

One management command, installed as a Go tool so its version is pinned in your `go.mod`:

```
go get -tool github.com/farhapartex/coyote/cmd/coyote
```

Run it from the directory holding your `main.go` and `settings.go`.

## Commands

| Command | What it does |
| --- | --- |
| `go tool coyote start` | Build and run the project, reporting migration and user state first |
| `go tool coyote makemigrations` | Diff your models against the snapshot and write a migration file |
| `go tool coyote migrate` | Apply migrations that have not been applied yet |
| `go tool coyote sqlmigrate` | Print the SQL a pending migration would run, without applying it |
| `go tool coyote createsuperadmin` | Create a superadmin who can sign in to the admin portal |
| `go tool coyote version` | Print the coyote version |
| `go tool coyote help` | List the commands |

## Flags

| Flag | Command | Purpose |
| --- | --- | --- |
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
go tool coyote makemigrations --name=initial
go tool coyote migrate
go tool coyote createsuperadmin
go tool coyote start
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

> `go coyote start` is not possible: the `go` command cannot be extended with new subcommands.

## Next

[First run →](24-first-run.md)
