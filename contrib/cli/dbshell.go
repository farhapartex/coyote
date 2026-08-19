package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/farhapartex/coyote/core/settings"
)

const NameDBShell = "dbshell"

type DBShell struct{}

func (DBShell) Name() string { return NameDBShell }

func (DBShell) Summary() string { return "open the database's own command line client" }

func (DBShell) Run(ctx Context) error {
	database := ctx.App.Config().Database()
	if database.Engine == "" {
		return fmt.Errorf("coyote/cli: no database configured in settings.go")
	}

	name, args, env, err := clientFor(database)
	if err != nil {
		return err
	}
	binary, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("coyote/cli: %s is not on PATH; install it to use dbshell", name)
	}

	fmt.Fprintf(ctx.Out, "%s %s\n", name, database.Redacted().DSN())

	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), env...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}

func clientFor(database settings.Database) (string, []string, []string, error) {
	switch database.Engine {
	case settings.SQLite:
		return "sqlite3", []string{database.Name}, nil, nil
	case settings.Postgres:
		args := []string{"--host", database.Host, "--dbname", database.Name}
		if database.Port > 0 {
			args = append(args, "--port", strconv.Itoa(database.Port))
		}
		if database.User != "" {
			args = append(args, "--username", database.User)
		}
		env := []string{}
		if database.Password != "" {
			env = append(env, "PGPASSWORD="+database.Password)
		}
		return "psql", args, env, nil
	case settings.MySQL:
		args := []string{"--host=" + database.Host, "--database=" + database.Name}
		if database.Port > 0 {
			args = append(args, "--port="+strconv.Itoa(database.Port))
		}
		if database.User != "" {
			args = append(args, "--user="+database.User)
		}
		env := []string{}
		if database.Password != "" {
			env = append(env, "MYSQL_PWD="+database.Password)
		}
		return "mysql", args, env, nil
	}
	return "", nil, nil, fmt.Errorf("coyote/cli: no client known for %s", database.Engine)
}
