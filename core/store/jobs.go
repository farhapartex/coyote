package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/lib/id"
	"gorm.io/gorm"
)

type JobOptions struct {
	MaxAttempts int
}

type jobQueue struct {
	resolve  Resolver
	defaults JobOptions
}

func Jobs(handle *gorm.DB, opts JobOptions) jobs.Queue {
	return LazyJobs(func() (*gorm.DB, error) { return handle, nil }, opts)
}

func LazyJobs(resolve Resolver, opts JobOptions) jobs.Queue {
	if opts.MaxAttempts == 0 {
		opts.MaxAttempts = jobs.DefaultMaxAttempts
	}
	return &jobQueue{resolve: resolve, defaults: opts}
}

func (q *jobQueue) WithTx(tx *gorm.DB) jobs.Queue {
	if tx == nil {
		return q
	}
	return &jobQueue{resolve: func() (*gorm.DB, error) { return tx, nil }, defaults: q.defaults}
}

func (q *jobQueue) handle(ctx context.Context) (*gorm.DB, error) {
	if q.resolve == nil {
		return nil, errors.New("coyote/store: no database resolver configured")
	}
	handle, err := q.resolve()
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, errors.New("coyote/store: no database connection")
	}
	return handle.WithContext(ctx), nil
}

func (q *jobQueue) Enqueue(ctx context.Context, job jobs.Job) (string, error) {
	if err := job.Valid(); err != nil {
		return "", err
	}
	handle, err := q.handle(ctx)
	if err != nil {
		return "", err
	}

	record, err := q.record(job)
	if err != nil {
		return "", err
	}
	if err := handle.Create(&record).Error; err != nil {
		return "", translateJob(err)
	}
	return record.ID, nil
}

func (q *jobQueue) record(job jobs.Job) (jobs.Record, error) {
	generated, err := id.New()
	if err != nil {
		return jobs.Record{}, err
	}

	now := time.Now().UTC()
	runAt := job.RunAt
	if runAt.IsZero() {
		runAt = now
	}
	queue := job.Queue
	if queue == "" {
		queue = jobs.DefaultQueue
	}

	record := jobs.Record{
		ID:          generated,
		Queue:       queue,
		Kind:        job.Kind,
		State:       jobs.Queued,
		RunAt:       runAt.UTC(),
		Priority:    job.Priority,
		Payload:     job.Payload,
		MaxAttempts: q.attempts(job.MaxAttempts),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if job.Fingerprint != "" {
		fingerprint := job.Fingerprint
		record.Fingerprint = &fingerprint
	}
	return record, nil
}

func (q *jobQueue) attempts(requested int) int {
	if requested == 0 {
		return q.defaults.MaxAttempts
	}
	if requested < 0 {
		return jobs.Forever
	}
	return requested
}

func translateJob(err error) error {
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique") || strings.Contains(text, "duplicate") {
		return jobs.ErrDuplicateFingerprint
	}
	return fmt.Errorf("coyote/repo: writing job: %w", err)
}
