package cli

import (
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/lib/text"
)

const NameMakeMigrations = "makemigrations"

type MakeMigrations struct {
	Label   string
	Undo    bool
	NoInput bool
}

func (MakeMigrations) Name() string { return NameMakeMigrations }

func (MakeMigrations) Summary() string {
	return "write a migration file for changes to your models"
}

func MakeMigrationsFromEnv() MakeMigrations {
	return MakeMigrations{
		Label:   flagSet(EnvName),
		Undo:    flagBool(EnvUndo),
		NoInput: flagBool(EnvNoInput),
	}
}

func (m MakeMigrations) Run(ctx Context) error {
	if m.Undo {
		return m.undo(ctx)
	}

	cfg := ctx.App.Config()
	dir := cfg.Migrations.Dir

	schemas, err := schemasOf(ctx.App)
	if err != nil {
		return err
	}
	desired := migrate.SnapshotOf(schemas)

	previous, err := migrate.LoadSnapshot(dir)
	if err != nil {
		return err
	}

	fmt.Fprintf(ctx.Out, "models     %d registered\n", len(schemas))
	fmt.Fprintf(ctx.Out, "migrations %s\n\n", dir)

	change := migrate.Diff(previous, desired)
	if change.Empty() {
		fmt.Fprintln(ctx.Out, "no model changes detected")
		return nil
	}

	name := m.Label
	if name == "" {
		name = suggestName(change)
	}
	generated, err := migrate.Generate(dir, name, change)
	if err != nil {
		return err
	}
	if err := migrate.SaveSnapshot(dir, desired); err != nil {
		return err
	}

	fmt.Fprintf(ctx.Out, "created %s\n", generated.Path)
	for _, op := range change.Ops {
		fmt.Fprintf(ctx.Out, "    %s\n", op.Describe())
	}
	if len(generated.Warnings) > 0 {
		fmt.Fprintln(ctx.Out, "\nreview before applying:")
		for _, warning := range generated.Warnings {
			fmt.Fprintf(ctx.Out, "  ! %s\n", warning)
		}
	}
	reportRouting(ctx)
	fmt.Fprintln(ctx.Out, "\ncommit the migration and the snapshot, then run: coyote migrate")
	fmt.Fprintln(ctx.Out, "if this is your first migration, blank import the package once from your main package:")
	fmt.Fprintln(ctx.Out, "  _ \"your/module/migrations\"")
	return nil
}

func suggestName(change migrate.Change) string {
	for _, op := range change.Ops {
		if created, ok := op.(migrate.CreateTable); ok {
			return "create_" + created.Table.Name
		}
	}
	if len(change.Ops) > 0 {
		return text.Slugify(change.Ops[0].Describe())
	}
	return "auto"
}
