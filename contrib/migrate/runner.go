package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrChecksumMismatch = errors.New("coyote/migrate: a migration changed after it was applied")

type Runner struct {
	handle  *gorm.DB
	ledger  *Ledger
	dialect dialect.Dialect
	pool    []Migration
}

func NewRunner(handle *gorm.DB, engine settings.Engine, migrations []Migration) *Runner {
	quiet := handle
	if handle != nil {
		quiet = handle.Session(&gorm.Session{Logger: logger.Discard})
	}
	return &Runner{
		handle:  quiet,
		ledger:  NewLedger(quiet),
		dialect: dialect.For(engine),
		pool:    migrations,
	}
}

func (r *Runner) Dialect() dialect.Dialect { return r.dialect }

func (r *Runner) Prepare(ctx context.Context) error { return r.ledger.Ensure(ctx) }

func (r *Runner) Applied(ctx context.Context) ([]Applied, error) { return r.ledger.Applied(ctx) }

type State struct {
	LedgerExists bool
	Applied      int
	Pending      []Migration
	Declared     int
}

func (s State) FreshDatabase() bool { return !s.LedgerExists || s.Applied == 0 }

func (r *Runner) State(ctx context.Context) (State, error) {
	state := State{Declared: len(r.pool)}
	if !r.ledger.Exists(ctx) {
		state.Pending = append([]Migration{}, r.pool...)
		return state, nil
	}
	state.LedgerExists = true

	applied, err := r.ledger.Applied(ctx)
	if err != nil {
		return state, err
	}
	state.Applied = len(applied)

	recorded := make(map[string]bool, len(applied))
	for _, record := range applied {
		recorded[record.ID] = true
	}
	for _, m := range r.pool {
		if !recorded[m.ID] {
			state.Pending = append(state.Pending, m)
		}
	}
	return state, nil
}

func (r *Runner) Pending(ctx context.Context) ([]Migration, error) {
	applied, err := r.ledger.Applied(ctx)
	if err != nil {
		return nil, err
	}
	recorded := make(map[string]Applied, len(applied))
	for _, record := range applied {
		recorded[record.ID] = record
	}

	pending := make([]Migration, 0, len(r.pool))
	for _, m := range r.pool {
		record, ok := recorded[m.ID]
		if !ok {
			pending = append(pending, m)
			continue
		}
		if record.Checksum != "" && record.Checksum != m.Checksum() {
			return nil, fmt.Errorf("%w: %s was applied as %s but is now %s",
				ErrChecksumMismatch, m.ID, record.Checksum, m.Checksum())
		}
	}
	return pending, nil
}

func (r *Runner) Statements(m Migration) []string {
	out := make([]string, 0, len(m.Up))
	for _, op := range m.Up {
		if renderable, ok := op.(Statementer); ok {
			out = append(out, renderable.Statements(r.dialect)...)
			continue
		}
		out = append(out, "-- "+op.Describe())
	}
	return out
}

func (r *Runner) Apply(ctx context.Context, m Migration) error {
	started := time.Now()
	return r.handle.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, op := range m.Up {
			if err := r.applyOp(ctx, tx, op); err != nil {
				return fmt.Errorf("%s: %s: %w", m.ID, op.Describe(), err)
			}
		}
		return r.ledger.Record(ctx, tx, m, time.Since(started))
	})
}

func (r *Runner) applyOp(ctx context.Context, tx *gorm.DB, op Op) error {
	if runnable, ok := op.(Executor); ok {
		return runnable.Exec(ctx, tx)
	}
	renderable, ok := op.(Statementer)
	if !ok {
		return fmt.Errorf("operation cannot be executed")
	}
	for _, statement := range renderable.Statements(r.dialect) {
		if err := tx.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) Fake(ctx context.Context, m Migration) error {
	return r.handle.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.ledger.Record(ctx, tx, m, 0)
	})
}

func (r *Runner) TablesExist(ctx context.Context, m Migration) bool {
	for _, op := range m.Up {
		created, ok := op.(CreateTable)
		if !ok {
			continue
		}
		if !r.handle.WithContext(ctx).Migrator().HasTable(created.Table.Name) {
			return false
		}
	}
	return true
}

func (r *Runner) Upto(pending []Migration, target string) ([]Migration, error) {
	if target == "" {
		return pending, nil
	}
	for i, m := range pending {
		if m.ID == target || strings.HasPrefix(m.ID, target+"_") || strings.HasPrefix(m.ID, target) {
			return pending[:i+1], nil
		}
	}
	return nil, fmt.Errorf("coyote/migrate: no pending migration matches %q", target)
}
