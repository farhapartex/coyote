package cli

import (
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate"
)

type Migrate struct{}

func (Migrate) Name() string { return NameMigrate }

func (Migrate) Summary() string { return "apply migrations that have not been applied yet" }

func (Migrate) Run(ctx Context) error {
	cfg := ctx.App.Config()

	if err := RequireServer(cfg.Addr()); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "server    running on %s\n", cfg.Addr())

	database := cfg.Database()
	if database.Engine == "" {
		return errors.New("coyote/cli: no database configured in settings.go")
	}
	fmt.Fprintf(ctx.Out, "database  %s %s\n", database.Engine, database.Name)

	handle, err := ctx.App.DB()
	if err != nil {
		return err
	}

	runner := migrate.New(handle, ctx.App.Models())
	fmt.Fprintf(ctx.Out, "models    %d registered\n\n", len(runner.Models()))

	report, err := runner.Plan()
	if err != nil {
		return err
	}
	if report.Empty() {
		fmt.Fprintln(ctx.Out, "schema is up to date, nothing to apply")
		return nil
	}

	for _, change := range report.Changes {
		fmt.Fprintf(ctx.Out, "  %s ... ", change)
		if err := runner.ApplyChange(change); err != nil {
			fmt.Fprintln(ctx.Out, "failed")
			return err
		}
		fmt.Fprintln(ctx.Out, "ok")
	}
	fmt.Fprintf(ctx.Out, "\napplied %d change(s)\n", len(report.Changes))
	return nil
}
