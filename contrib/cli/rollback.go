package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/farhapartex/coyote/contrib/migrate"
)

const NameRollback = "rollback"

type Rollback struct {
	Steps   int
	Force   bool
	NoInput bool
}

func (Rollback) Name() string { return NameRollback }

func (Rollback) Summary() string { return "undo the most recently applied migration" }

func RollbackFromEnv() Rollback {
	return Rollback{
		Steps:   flagInt(EnvSteps, 1),
		Force:   flagBool(EnvForce),
		NoInput: flagBool(EnvNoInput),
	}
}

func (c Rollback) Run(ctx Context) error {
	cfg := ctx.App.Config()
	database := cfg.Database()
	if database.Engine == "" {
		return errors.New("coyote/cli: no database configured in settings.go")
	}

	handle, err := ctx.App.DB()
	if err != nil {
		return err
	}
	if handle == nil {
		return fmt.Errorf("coyote/cli: no connection to %s %s", database.Engine, database.Name)
	}

	runner := migrate.NewRunner(handle, database.Engine, migrate.Registered())
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		return err
	}

	plan, err := runner.PlanRollback(background, c.Steps)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Fprintln(ctx.Out, "nothing to roll back")
		return nil
	}

	fmt.Fprintf(ctx.Out, "database  %s %s\n\n", database.Engine, database.Name)
	losses := []string{}
	for _, entry := range plan {
		fmt.Fprintf(ctx.Out, "  %s\n", entry.Migration.ID)
		for _, op := range entry.Ops {
			fmt.Fprintf(ctx.Out, "      %s\n", op.Describe())
		}
		for _, blocked := range entry.Blocked {
			fmt.Fprintf(ctx.Out, "      ! cannot reverse: %s\n", blocked)
		}
		losses = append(losses, entry.Losses...)
	}

	for _, entry := range plan {
		if len(entry.Blocked) > 0 && !c.Force {
			return fmt.Errorf("coyote/cli: %s has operations that cannot be reversed; write a Down in the migration, or pass --force to run the rest",
				entry.Migration.ID)
		}
	}

	if len(losses) > 0 {
		fmt.Fprintln(ctx.Out, "\nthis will destroy:")
		for _, loss := range losses {
			fmt.Fprintf(ctx.Out, "  ! %s\n", loss)
		}
		if !c.NoInput && !confirmed(ctx) {
			fmt.Fprintln(ctx.Out, "\nleft alone")
			return nil
		}
	}

	fmt.Fprintln(ctx.Out)
	err = runner.WithLock(background, func() error {
		for _, entry := range plan {
			if err := runner.Undo(background, entry); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "  rolled back %s\n", entry.Migration.ID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "\nrolled back %d migration(s)\n", len(plan))
	return nil
}

func confirmed(ctx Context) bool {
	fmt.Fprint(ctx.Out, "\ntype yes to continue: ")
	reader := bufio.NewReader(ctx.Input())
	answer, err := reader.ReadString('\n')
	if err != nil && answer == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(answer), "yes")
}
