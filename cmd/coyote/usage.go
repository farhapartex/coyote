package main

const usage = `coyote — the Coyote management command

usage:
  go tool coyote <command> [flags]

commands:
  start      build and run the project in the current directory
  migrate    apply migrations that have not been applied yet
  version    print the coyote version
  help       print this message

flags for start:
  --port=N   listen on port N instead of the one in settings.go
  --host=H   bind to host H instead of the one in settings.go

run these from the directory holding your main package and settings.go.
`
