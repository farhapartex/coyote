package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
)

var ErrLockBusy = errors.New("coyote/migrate: another process is migrating this database")

func (r *Runner) WithLock(ctx context.Context, fn func() error) error {
	locker, ok := r.dialect.(dialect.Locker)
	if !ok {
		return fn()
	}

	pool, err := r.handle.DB()
	if err != nil {
		return err
	}
	conn, err := pool.Conn(ctx)
	if err != nil {
		return fmt.Errorf("coyote/migrate: reserving a connection for the lock: %w", err)
	}
	defer conn.Close()

	acquire, release := locker.AdvisoryLock()
	var held sql.NullBool
	if err := conn.QueryRowContext(ctx, acquire).Scan(&held); err != nil {
		return fmt.Errorf("coyote/migrate: taking the migration lock: %w", err)
	}
	if !held.Valid || !held.Bool {
		return ErrLockBusy
	}
	defer conn.ExecContext(ctx, release)

	return fn()
}

func (r *Runner) TransactionalDDL() bool {
	atomic, ok := r.dialect.(dialect.Atomic)
	if !ok {
		return true
	}
	return atomic.TransactionalDDL()
}
