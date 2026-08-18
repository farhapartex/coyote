package migrate

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	if r.needsRebuild(m.Up) {
		return r.applyOutsideTransaction(ctx, m)
	}

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

func (r *Runner) needsRebuild(ops []Op) bool {
	for _, op := range ops {
		if alter, ok := op.(AlterColumn); ok && alter.Rebuilds(r.dialect) {
			return true
		}
	}
	return false
}

func (r *Runner) applyOutsideTransaction(ctx context.Context, m Migration) error {
	started := time.Now()
	handle := r.handle.WithContext(ctx)

	if err := handle.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return err
	}
	defer handle.Exec("PRAGMA foreign_keys = ON")

	for _, op := range m.Up {
		if err := r.applyOp(ctx, handle, op); err != nil {
			return fmt.Errorf("%s: %s: %w", m.ID, op.Describe(), err)
		}
	}

	violations := []map[string]any{}
	if err := handle.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("%s: the rebuild left %d foreign key violation(s); the database was not recorded as migrated",
			m.ID, len(violations))
	}
	return r.ledger.Record(ctx, handle, m, time.Since(started))
}

func (r *Runner) applyOp(ctx context.Context, tx *gorm.DB, op Op) error {
	if alter, ok := op.(AlterColumn); ok && alter.Rebuilds(r.dialect) {
		statements, err := alter.rebuildStatements(r.dialect)
		if err != nil {
			return err
		}
		for _, statement := range statements {
			if err := tx.WithContext(ctx).Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	}
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

type Rollback struct {
	Migration Migration
	Ops       []Op
	Blocked   []string
	Losses    []string
}

func (r *Runner) PlanRollback(ctx context.Context, steps int) ([]Rollback, error) {
	applied, err := r.ledger.Applied(ctx)
	if err != nil {
		return nil, err
	}
	if steps < 1 {
		steps = 1
	}

	declared := make(map[string]Migration, len(r.pool))
	for _, m := range r.pool {
		declared[m.ID] = m
	}

	sort.Slice(applied, func(i, j int) bool { return applied[i].ID > applied[j].ID })

	out := []Rollback{}
	for _, record := range applied {
		if len(out) == steps {
			break
		}
		m, known := declared[record.ID]
		if !known {
			return nil, fmt.Errorf("coyote/migrate: %s is applied but not declared; it cannot be rolled back", record.ID)
		}
		if len(m.Replaces) > 0 {
			return nil, fmt.Errorf("coyote/migrate: %s replaces %d migration(s); rollback stops at a squash boundary",
				m.ID, len(m.Replaces))
		}
		ops, blocked := m.Reverse()
		out = append(out, Rollback{Migration: m, Ops: ops, Blocked: blocked, Losses: Destructive(ops)})
	}
	return out, nil
}

func (r *Runner) Undo(ctx context.Context, plan Rollback) error {
	if len(plan.Blocked) > 0 {
		return fmt.Errorf("coyote/migrate: %s cannot be reversed: %s",
			plan.Migration.ID, strings.Join(plan.Blocked, ", "))
	}
	return r.handle.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, op := range plan.Ops {
			if err := r.applyOp(ctx, tx, op); err != nil {
				return fmt.Errorf("%s: %s: %w", plan.Migration.ID, op.Describe(), err)
			}
		}
		return r.ledger.Forget(ctx, tx, plan.Migration.ID)
	})
}
