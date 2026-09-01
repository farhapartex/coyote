package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/farhapartex/coyote/contrib/migrate"
)

func (m MakeMigrations) undo(ctx Context) error {
	cfg := ctx.App.Config()
	dir := cfg.Migrations.Dir

	pool := migrate.Registered()
	if len(pool) == 0 {
		fmt.Fprintln(ctx.Out, "no migrations are declared, so there is nothing to undo")
		return nil
	}
	last := pool[len(pool)-1]

	applied, err := alreadyApplied(ctx, last)
	if err != nil {
		return err
	}
	if applied {
		return fmt.Errorf("coyote/cli: %s has already been applied; roll it back first with: coyote rollback", last.ID)
	}

	ops, blocked := last.Reverse()
	if len(blocked) > 0 {
		return fmt.Errorf("coyote/cli: %s cannot be undone because it has operations that do not reverse: %s",
			last.ID, strings.Join(blocked, ", "))
	}

	previous, err := migrate.LoadSnapshot(dir)
	if err != nil {
		return err
	}
	rewound, err := previous.Apply(ops)
	if err != nil {
		return fmt.Errorf("coyote/cli: %s does not undo cleanly against %s: %w", last.ID, migrate.SnapshotFile, err)
	}

	path := filepath.Join(dir, last.ID+".go")
	fmt.Fprintf(ctx.Out, "undo %s\n\n", last.ID)
	for _, op := range last.Up {
		fmt.Fprintf(ctx.Out, "    %s\n", op.Describe())
	}
	fmt.Fprintf(ctx.Out, "\nthis deletes %s and rewinds %s\n", path, migrate.SnapshotFile)

	if !m.NoInput && !confirmed(ctx) {
		fmt.Fprintln(ctx.Out, "\nleft alone")
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("coyote/cli: removing %s: %w", path, err)
	}
	if err := migrate.SaveSnapshot(dir, rewound); err != nil {
		return err
	}

	fmt.Fprintf(ctx.Out, "\nremoved %s and rewound %s\n", filepath.Base(path), migrate.SnapshotFile)
	return nil
}

func alreadyApplied(ctx Context, m migrate.Migration) (bool, error) {
	cfg := ctx.App.Config()
	database := cfg.Database()
	if database.Engine == "" {
		return false, fmt.Errorf("coyote/cli: no database configured in settings.go, so %s cannot be checked for being applied", m.ID)
	}

	handle, err := ctx.App.DB()
	if err != nil {
		return false, err
	}
	if handle == nil {
		return false, fmt.Errorf("coyote/cli: no connection to %s %s", database.Engine, database.Name)
	}

	runner := migrate.NewRunner(handle, database.Engine, migrate.Registered())
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		return false, err
	}
	pending, err := runner.Pending(background)
	if err != nil {
		return false, err
	}
	for _, entry := range pending {
		if entry.ID == m.ID {
			return false, nil
		}
	}
	return true, nil
}
