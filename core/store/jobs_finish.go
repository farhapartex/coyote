package store

import (
	"context"
	"fmt"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
)

const maxStoredError = 2000

func (q *jobQueue) Complete(ctx context.Context, jobID string, at time.Time) error {
	handle, err := q.handle(ctx)
	if err != nil {
		return err
	}

	stamp := at.UTC()
	result := handle.Model(&jobs.Record{}).
		Where("id = ? AND state = ?", jobID, jobs.Running).
		Updates(map[string]any{
			"state":       jobs.Done,
			"locked_by":   "",
			"locked_at":   nil,
			"last_error":  "",
			"finished_at": stamp,
			"updated_at":  stamp,
		})
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: completing job %s: %w", jobID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s is not running", jobs.ErrNotFound, jobID)
	}
	return nil
}

func (q *jobQueue) Fail(ctx context.Context, jobID, cause string, retryAt *time.Time) error {
	handle, err := q.handle(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	changes := map[string]any{
		"locked_by":  "",
		"locked_at":  nil,
		"last_error": truncateError(cause),
		"updated_at": now,
	}
	if retryAt != nil {
		changes["state"] = jobs.Queued
		changes["run_at"] = retryAt.UTC()
	} else {
		changes["state"] = jobs.Dead
		changes["finished_at"] = now
	}

	result := handle.Model(&jobs.Record{}).
		Where("id = ? AND state = ?", jobID, jobs.Running).
		Updates(changes)
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: failing job %s: %w", jobID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s is not running", jobs.ErrNotFound, jobID)
	}
	return nil
}

func (q *jobQueue) Retry(ctx context.Context, jobID string, at time.Time) error {
	handle, err := q.handle(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	result := handle.Model(&jobs.Record{}).
		Where("id = ? AND state IN ?", jobID, []jobs.State{jobs.Dead, jobs.Done}).
		Updates(map[string]any{
			"state":       jobs.Queued,
			"run_at":      at.UTC(),
			"attempts":    0,
			"locked_by":   "",
			"locked_at":   nil,
			"last_error":  "",
			"finished_at": nil,
			"updated_at":  now,
		})
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: retrying job %s: %w", jobID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s has not finished, so there is nothing to retry", jobs.ErrNotFound, jobID)
	}
	return nil
}

func (q *jobQueue) Sweep(ctx context.Context, finishedBefore time.Time) (int, error) {
	handle, err := q.handle(ctx)
	if err != nil {
		return 0, err
	}

	result := handle.
		Where("state = ? AND finished_at IS NOT NULL AND finished_at < ?", jobs.Done, finishedBefore.UTC()).
		Delete(&jobs.Record{})
	if result.Error != nil {
		return 0, fmt.Errorf("coyote/repo: sweeping finished jobs: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

func (q *jobQueue) Stats(ctx context.Context) (jobs.Stats, error) {
	handle, err := q.handle(ctx)
	if err != nil {
		return jobs.Stats{}, err
	}

	type tally struct {
		State jobs.State
		Total int64
	}
	rows := []tally{}
	err = handle.Model(&jobs.Record{}).
		Select("state, count(*) as total").
		Group("state").
		Find(&rows).Error
	if err != nil {
		return jobs.Stats{}, fmt.Errorf("coyote/repo: counting jobs: %w", err)
	}

	out := jobs.Stats{}
	for _, row := range rows {
		switch row.State {
		case jobs.Queued:
			out.Queued = row.Total
		case jobs.Running:
			out.Running = row.Total
		case jobs.Done:
			out.Done = row.Total
		case jobs.Dead:
			out.Dead = row.Total
		}
	}
	return out, nil
}

func truncateError(cause string) string {
	if len(cause) <= maxStoredError {
		return cause
	}
	return cause[:maxStoredError]
}
