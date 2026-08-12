package main

import (
	"fmt"
	"os"

	"github.com/farhapartex/coyote/contrib/cli"
)

const version = "0.4.0"

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
		fmt.Printf("coyote %s\n", version)
		return nil
	case cli.NameStart:
		return start(rest)
	case cli.NameMigrate:
		return migrate(rest)
	case cli.NameMakeMigrations:
		return makeMigrations(rest)
	case cli.NameSQLMigrate:
		return sqlMigrate(rest)
	case cli.NameCreateSuperadmin:
		return createSuperadmin(rest)
	default:
		fmt.Print(usage)
		return fmt.Errorf("unknown command %q", command)
	}
}
