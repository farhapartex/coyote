package main

import (
	"fmt"
	"os"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/app"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "coyote: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	command, rest := args[0], args[1:]
	switch command {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version", "--version":
		fmt.Printf("coyote %s\n", app.Version)
		return nil
	case "new":
		return newProject(rest)
	case "startapp":
		return startApp(rest)
	case cli.NameStart:
		return start(rest)
	case cli.NameMigrate:
		return migrate(rest)
	case cli.NameRollback:
		return rollback(rest)
	case cli.NameSyncPermissions:
		return syncPermissions(rest)
	case cli.NameShell:
		return simple(cli.NameShell, rest)
	case cli.NameDBShell:
		return simple(cli.NameDBShell, rest)
	case cli.NameWorker:
		return worker(rest)
	case cli.NameCollectStatic:
		return collectStatic(rest)
	case cli.NameMakeMigrations:
		return makeMigrations(rest)
	case cli.NameMakeMessages:
		return makeMessages(rest)
	case cli.NameCheckMessages:
		return checkMessages(rest)
	case cli.NameSQLMigrate:
		return sqlMigrate(rest)
	case cli.NameCreateSuperadmin:
		return createSuperadmin(rest)
	default:
		return passThrough(command, rest)
	}
}
