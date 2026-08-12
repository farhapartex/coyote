package cli

import (
	"context"
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

	pool := migrate.Registered()
	runner := migrate.NewRunner(handle, database.Engine, pool)

	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		return err
	}
	pending, err := runner.Pending(background)
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "migrations %d found, %d pending\n\n", len(pool), len(pending))

	if len(pending) == 0 {
		if len(pool) == 0 {
			fmt.Fprintln(ctx.Out, "no migrations declared; run: go tool coyote makemigrations")
			return nil
		}
		fmt.Fprintln(ctx.Out, "database is up to date, nothing to apply")
		return nil
	}

	for _, m := range pending {
		fmt.Fprintf(ctx.Out, "  %s\n", m.Title())
		for _, step := range m.Describe() {
			fmt.Fprintf(ctx.Out, "      %s\n", step)
		}
		if err := runner.Apply(background, m); err != nil {
			fmt.Fprintln(ctx.Out, "      failed")
			return err
		}
		fmt.Fprintln(ctx.Out, "      applied")
	}
	fmt.Fprintf(ctx.Out, "\napplied %d migration(s)\n", len(pending))
	return nil
}
