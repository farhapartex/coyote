package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate"
)

const NameSQLMigrate = "sqlmigrate"

type SQLMigrate struct{}

func (SQLMigrate) Name() string { return NameSQLMigrate }

func (SQLMigrate) Summary() string {
	return "print the SQL a pending migration would run, without applying it"
}

func (SQLMigrate) Run(ctx Context) error {
	database := ctx.App.Config().Database()
	handle, err := ctx.App.DB()
	if err != nil {
		return err
	}
	if handle == nil {
		return errors.New("coyote/cli: no database connection")
	}

	runner := migrate.NewRunner(handle, database.Engine, migrate.Registered())
	if err := runner.Prepare(context.Background()); err != nil {
		return err
	}
	pending, err := runner.Pending(context.Background())
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Fprintln(ctx.Out, "-- nothing pending")
		return nil
	}
	for _, m := range pending {
		fmt.Fprintf(ctx.Out, "-- %s (%s)\n", m.ID, runner.Dialect().Name())
		for _, statement := range runner.Statements(m) {
			fmt.Fprintf(ctx.Out, "%s;\n", statement)
		}
		fmt.Fprintln(ctx.Out)
	}
	return nil
}
