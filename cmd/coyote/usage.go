package main

const usage = `coyote — the Coyote management command

usage:
  go tool coyote <command> [flags]

commands:
  start           build and run the project in the current directory
  makemigrations  write a migration file for changes to your models
  migrate         apply migrations that have not been applied yet
  sqlmigrate      print the SQL a pending migration would run
  createsuperadmin  create a superadmin who can sign in to the admin portal
  version         print the coyote version
  help            print this message

flags for start:
  --port=N        listen on port N instead of the one in settings.go
  --host=H        bind to host H instead of the one in settings.go

flags for makemigrations:
  --name=NAME     name the generated migration instead of guessing one

flags for createsuperadmin:
  --username=U    skip the prompt for the username
  --email=E       skip the prompt for the email
  --password=P    skip the prompt for the password

createsuperadmin also reads COYOTE_SUPERADMIN_USERNAME, _EMAIL and _PASSWORD,
which is the safer route in scripts.

run these from the directory holding your main package and settings.go.
`
