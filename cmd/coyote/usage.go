package main

const usage = `coyote — the Coyote management command

usage:
  coyote <command> [flags]

commands:
  new <name>        create a new project in a directory of that name
  start             build and run the project in the current directory
  makemigrations    write a migration file for changes to your models
  migrate           apply migrations that have not been applied yet
  sqlmigrate        print the SQL a pending migration would run
  syncpermissions   create the permissions for every registered model
  collectstatic     fingerprint static files and write a manifest
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

flags for makemigrations:
  --name=NAME       name the generated migration instead of guessing one

flags for createsuperadmin:
  --username=U      skip the prompt for the username
  --email=E         skip the prompt for the email
  --password=P      skip the prompt for the password

createsuperadmin also reads COYOTE_SUPERADMIN_USERNAME, _EMAIL and _PASSWORD,
which is the safer route in scripts.

every command except "new" runs from the directory holding your main package
and settings.go. installed per project, the same commands are available as
"go tool coyote <command>".
`
