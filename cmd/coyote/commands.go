package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/farhapartex/coyote/contrib/cli"
)

func start(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	port := fs.Int("port", 0, "port to listen on")
	host := fs.String("host", "", "host to bind")
	if err := fs.Parse(args); err != nil {
		return err
	}

	env := os.Environ()
	env = append(env, cli.EnvCommand+"="+cli.NameStart)
	if *port != 0 {
		if *port < 1 || *port > 65535 {
			return fmt.Errorf("--port must be between 1 and 65535, got %d", *port)
		}
		env = append(env, cli.EnvPort+"="+strconv.Itoa(*port))
	}
	if *host != "" {
		env = append(env, cli.EnvHost+"="+*host)
	}
	return invoke(env)
}

func migrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fake := fs.Bool("fake", false, "record migrations as applied without running them")
	fakeInitial := fs.Bool("fake-initial", false, "record only the first migration, and only if its tables exist")
	target := fs.String("to", "", "stop after this migration")
	if err := fs.Parse(args); err != nil {
		return err
	}

	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameMigrate)
	switch {
	case *fakeInitial:
		env = append(env, cli.EnvFake+"=initial")
	case *fake:
		env = append(env, cli.EnvFake+"=1")
	}
	if *target != "" {
		env = append(env, cli.EnvTarget+"="+*target)
	}
	return invoke(env)
}

func rollback(args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	steps := fs.Int("steps", 1, "how many migrations to undo")
	force := fs.Bool("force", false, "roll back even when some operations cannot be reversed")
	noInput := fs.Bool("no-input", false, "do not ask for confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}

	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameRollback)
	env = append(env, cli.EnvSteps+"="+strconv.Itoa(*steps))
	if *force {
		env = append(env, cli.EnvForce+"=1")
	}
	if *noInput {
		env = append(env, cli.EnvNoInput+"=1")
	}
	return invoke(env)
}

func syncPermissions(args []string) error {
	fs := flag.NewFlagSet("syncpermissions", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return invoke(append(os.Environ(), cli.EnvCommand+"="+cli.NameSyncPermissions))
}

func collectStatic(args []string) error {
	fs := flag.NewFlagSet("collectstatic", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return invoke(append(os.Environ(), cli.EnvCommand+"="+cli.NameCollectStatic))
}

func makeMigrations(args []string) error {
	fs := flag.NewFlagSet("makemigrations", flag.ContinueOnError)
	name := fs.String("name", "", "name for the generated migration")
	undo := fs.Bool("undo", false, "delete the most recent migration if it has not been applied, and rewind the snapshot")
	noInput := fs.Bool("no-input", false, "do not ask for confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameMakeMigrations)
	if *name != "" {
		env = append(env, cli.EnvName+"="+*name)
	}
	if *undo {
		env = append(env, cli.EnvUndo+"=1")
	}
	if *noInput {
		env = append(env, cli.EnvNoInput+"=1")
	}
	return invoke(env)
}

func makeMessages(args []string) error {
	fs := flag.NewFlagSet("makemessages", flag.ContinueOnError)
	locale := fs.String("locale", "", "only update this locale")
	if err := fs.Parse(args); err != nil {
		return err
	}
	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameMakeMessages)
	if *locale != "" {
		env = append(env, cli.EnvLocale+"="+*locale)
	}
	return invoke(env)
}

func checkMessages(args []string) error {
	fs := flag.NewFlagSet("checkmessages", flag.ContinueOnError)
	locale := fs.String("locale", "", "only check this locale")
	strict := fs.Bool("strict", false, "fail on fuzzy and obsolete entries too")
	if err := fs.Parse(args); err != nil {
		return err
	}
	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameCheckMessages)
	if *locale != "" {
		env = append(env, cli.EnvLocale+"="+*locale)
	}
	if *strict {
		env = append(env, cli.EnvStrict+"=1")
	}
	return invoke(env)
}

func createSuperadmin(args []string) error {
	fs := flag.NewFlagSet("createsuperadmin", flag.ContinueOnError)
	username := fs.String("username", "", "username for the superadmin")
	email := fs.String("email", "", "email for the superadmin")
	password := fs.String("password", "", "password for the superadmin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameCreateSuperadmin)
	for key, value := range map[string]string{
		cli.EnvUsername: *username,
		cli.EnvEmail:    *email,
		cli.EnvPassword: *password,
	} {
		if value != "" {
			env = append(env, key+"="+value)
		}
	}
	return invoke(env)
}

func sqlMigrate(args []string) error {
	fs := flag.NewFlagSet("sqlmigrate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return invoke(append(os.Environ(), cli.EnvCommand+"="+cli.NameSQLMigrate))
}

func simple(name string, args []string) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return invoke(append(os.Environ(), cli.EnvCommand+"="+name))
}

func passThrough(command string, args []string) error {
	if strings.HasPrefix(command, "-") {
		fmt.Print(usage)
		return fmt.Errorf("unknown flag %q", command)
	}

	env := append(os.Environ(), cli.EnvCommand+"="+command)
	if len(args) > 0 {
		env = append(env, cli.EnvArgs+"="+strings.Join(args, " "))
	}
	return invoke(env)
}
