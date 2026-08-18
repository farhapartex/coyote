package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate"
)

type Migrate struct {
	Fake        bool
	FakeInitial bool
	Target      string
}

func (Migrate) Name() string { return NameMigrate }

func (Migrate) Summary() string { return "apply migrations that have not been applied yet" }

func MigrateFromEnv() Migrate {
	return Migrate{
		Fake:        flagSet(EnvFake) == "1",
		FakeInitial: flagSet(EnvFake) == "initial",
		Target:      flagSet(EnvTarget),
	}
}

func (m Migrate) Run(ctx Context) error {
	cfg := ctx.App.Config()

	database := cfg.Database()
	if database.Engine == "" {
		return errors.New("coyote/cli: no database configured in settings.go")
	}
	fmt.Fprintf(ctx.Out, "database  %s %s\n", database.Engine, database.Name)

	handle, err := ctx.App.DB()
	if err != nil {
		return err
	}
	if handle == nil {
		return fmt.Errorf("coyote/cli: no connection to %s %s", database.Engine, database.Name)
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
	if pending, err = runner.Upto(pending, m.Target); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "migrations %d found, %d pending\n\n", len(pool), len(pending))

	if len(pending) == 0 {
		if len(pool) == 0 {
			fmt.Fprintln(ctx.Out, "no migrations declared; run: coyote makemigrations")
			return nil
		}
		fmt.Fprintln(ctx.Out, "database is up to date, nothing to apply")
		afterMigrate(ctx)
		return nil
	}

	if m.Fake || m.FakeInitial {
		return m.fake(ctx, runner, pending)
	}

	for _, entry := range pending {
		fmt.Fprintf(ctx.Out, "  %s\n", entry.Title())
		for _, step := range entry.Describe() {
			fmt.Fprintf(ctx.Out, "      %s\n", step)
		}
		if err := runner.Apply(background, entry); err != nil {
			fmt.Fprintln(ctx.Out, "      failed")
			return err
		}
		fmt.Fprintln(ctx.Out, "      applied")
	}
	fmt.Fprintf(ctx.Out, "\napplied %d migration(s)\n", len(pending))
	afterMigrate(ctx)
	return nil
}

func (m Migrate) fake(ctx Context, runner *migrate.Runner, pending []migrate.Migration) error {
	background := context.Background()

	if m.FakeInitial {
		if len(pending) == 0 {
			return nil
		}
		first := pending[0]
		if !runner.TablesExist(background, first) {
			return fmt.Errorf("coyote/cli: --fake-initial needs the tables from %s to exist already", first.ID)
		}
		pending = pending[:1]
	}

	fmt.Fprintln(ctx.Out, "recording as applied WITHOUT running anything:")
	for _, entry := range pending {
		if err := runner.Fake(background, entry); err != nil {
			return err
		}
		fmt.Fprintf(ctx.Out, "  faked %s\n", entry.ID)
	}
	fmt.Fprintf(ctx.Out, "\nfaked %d migration(s); the database was not changed\n", len(pending))
	return nil
}

func afterMigrate(ctx Context) {
	reportRouting(ctx)

	report, err := syncPermissions(ctx)
	if err != nil {
		fmt.Fprintf(ctx.Out, "\npermissions were not synced: %v\n", err)
		return
	}
	if len(report.Created) == 0 && len(report.Stale) == 0 {
		return
	}
	fmt.Fprintln(ctx.Out)
	reportPermissions(ctx, report)
}
