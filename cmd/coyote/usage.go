package main

const usage = `coyote — the Coyote management command

usage:
  coyote <command> [flags]

commands:
  new <name>        create a new project in a directory of that name
  startapp <name>   scaffold a feature package under internal/
  start             build and run the project in the current directory
  makemigrations    write a migration file for changes to your models
  makemessages      extract translatable text into your catalogs
  checkmessages     report missing, fuzzy or obsolete translations
  migrate           apply migrations that have not been applied yet
  sqlmigrate        print the SQL a pending migration would run
  rollback          undo the most recently applied migration
  syncpermissions   create the permissions for every registered model
  collectstatic     fingerprint static files and write a manifest
  shell             inspect and query your models interactively
  dbshell           open the database's own command line client
  createsuperadmin  create a superadmin who can sign in to the admin portal
  version           print the coyote version
  help              print this message

flags for new:
  --module=PATH     module path for go.mod, defaults to the project name
  --force           scaffold into a directory that is not empty
  --skip-deps       write the files without running the go toolchain

flags for start:
  --port=N          listen on port N instead of the one in settings.go
  --host=H          bind to host H instead of the one in settings.go

flags for migrate:
  --fake            record pending migrations as applied without running them
  --fake-initial    record only the first migration, if its tables already exist
  --to=ID           stop after migration ID

flags for rollback:
  --steps=N         undo N migrations instead of one
  --force           proceed even when some operations cannot be reversed
  --no-input        do not ask before destroying data

flags for makemigrations:
  --name=NAME       name the generated migration instead of guessing one

flags for makemessages and checkmessages:
  --locale=TAG      work on one locale instead of every supported one
  --strict          checkmessages only: fail on fuzzy and obsolete entries too

flags for createsuperadmin:
  --username=U      skip the prompt for the username
  --email=E         skip the prompt for the email
  --password=P      skip the prompt for the password

createsuperadmin also reads COYOTE_SUPERADMIN_USERNAME, _EMAIL and _PASSWORD,
which is the safer route in scripts.

any command your application registers with cli.Register is available here too.

every command except "new" runs from the directory holding your main package
and settings.go. installed per project, the same commands are available as
"go tool coyote <command>".
`
