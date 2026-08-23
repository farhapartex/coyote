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
| `coyote makemessages` | Extract translatable text into your catalogs |
| `coyote checkmessages` | Report missing, fuzzy or obsolete translations |
| `coyote syncpermissions` | Create the four permissions for every registered model |
| `coyote collectstatic` | Fingerprint static files and write a manifest |
| `coyote rollback` | Undo the most recently applied migration |
| `coyote startapp <name>` | Scaffold a feature package under `internal/` |
| `coyote shell` | Inspect and query your models interactively |
| `coyote dbshell` | Open the database's own command line client |
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
| `--locale=TAG` | `makemessages`, `checkmessages` | One locale instead of every supported one |
| `--strict` | `checkmessages` | Also fail on fuzzy and obsolete entries |
| `--username=U` | `createsuperadmin` | Skip the username prompt |
| `--email=E` | `createsuperadmin` | Skip the email prompt |
| `--password=P` | `createsuperadmin` | Skip the password prompt |

`createsuperadmin` also reads `COYOTE_SUPERADMIN_USERNAME`, `_EMAIL` and `_PASSWORD`, which is what
you want in scripts.

**The password prompt is hidden** when you are on a terminal, and asks twice to catch typos. When
input is piped or redirected it falls back to reading a line normally, so automation keeps working.

## A new project

```
coyote new myshop
cd myshop
coyote makemigrations --name=initial
coyote migrate
coyote createsuperadmin
coyote start
```

## Two more shells

```
coyote shell
```

An interactive prompt over your **models** — not a Go REPL, since Go has no interpreter in the
standard library:

```
> .models
products
users
> count products
12
> list products 3
  id=7f2… name=Kettle price=19.5
> get products 7f2c9a11-…
  id=7f2… name=Kettle price=19.5
> .describe products
  id      string, primary key
  name    string, required
> .quit
```

Mistakes are reported and the prompt stays open. `.help` lists everything.

```
coyote dbshell
```

Hands you the engine's own client — `sqlite3`, `psql` or `mysql` — with the connection from your
settings already applied. Passwords go through `PGPASSWORD`/`MYSQL_PWD` rather than the command line,
so they do not show up in your shell history or in `ps`. If the client is not installed, the error
says which one to install.

## Adding your own commands

Register them before `Run`, and they dispatch exactly like the built-ins:

```go
type reindex struct{}

func (reindex) Name() string    { return "reindex" }
func (reindex) Summary() string { return "rebuild the search index" }

func (reindex) Run(ctx cli.Context) error {
	fmt.Fprintln(ctx.Out, "reindexing", cli.Args())
	return nil
}

func main() {
	cli.Register(reindex{})

	a := app.New()
	log.Fatal(a.Run())
}
```

```
coyote reindex products
```

`coyote` forwards any subcommand it does not recognise to your application, so the outer binary needs
no knowledge of your commands. Arguments arrive through `cli.Args()`.

`ctx` carries the application (`ctx.App`), an output writer, and stdin — plus `ctx.ReadSecret` for
hidden input and `ctx.Interactive()` to tell a terminal from a pipe.

## Starting a feature package

```
coyote startapp invoices
```

```
internal/invoices/models.go      an entity to edit
internal/invoices/admin.go       its admin resource
internal/invoices/handlers.go    a list handler
internal/invoices/register.go    Models(), Routes(a), Manage(portal)
templates/pages/invoices.html    a page to render
```

A plural name gives a singular type — `invoices` becomes `Invoice`. The command prints the three lines
to paste into `main.go`, and refuses rather than overwriting an existing package or page.

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
