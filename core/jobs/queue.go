package jobs

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Queue interface {
	Enqueue(ctx context.Context, job Job) (string, error)
	Claim(ctx context.Context, worker string, queues []string, limit int) ([]Record, error)
	Complete(ctx context.Context, id string, at time.Time) error
	Fail(ctx context.Context, id string, cause string, retryAt *time.Time) error
	Retry(ctx context.Context, id string, at time.Time) error
	Heartbeat(ctx context.Context, id string, at time.Time) error
	Recover(ctx context.Context, lockedBefore time.Time) (int, error)
	Sweep(ctx context.Context, finishedBefore time.Time) (int, error)
	Stats(ctx context.Context) (Stats, error)
	WithTx(tx *gorm.DB) Queue
}
