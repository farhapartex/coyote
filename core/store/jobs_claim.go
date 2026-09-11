package store

import (
	"context"
	"fmt"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
	"gorm.io/gorm"
)

const claimOverscan = 2

func (q *jobQueue) Claim(ctx context.Context, worker string, queues []string, limit int) ([]jobs.Record, error) {
	if limit < 1 {
		return nil, nil
	}
	handle, err := q.handle(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	candidates, err := q.candidates(handle, queues, now, limit*claimOverscan)
	if err != nil {
		return nil, err
	}

	claimed := make([]jobs.Record, 0, limit)
	for _, candidate := range candidates {
		if len(claimed) == limit {
			break
		}
		taken, err := q.take(handle, candidate.ID, worker, now)
		if err != nil {
			return claimed, err
		}
		if !taken {
			continue
		}
		claimed = append(claimed, running(candidate, worker, now))
	}
	return claimed, nil
}

func (q *jobQueue) candidates(handle *gorm.DB, queues []string, now time.Time, limit int) ([]jobs.Record, error) {
	session := handle.Model(&jobs.Record{}).
		Where("state = ? AND run_at <= ?", jobs.Queued, now)
	if len(queues) > 0 {
		session = session.Where("queue IN ?", queues)
	}

	rows := []jobs.Record{}
	err := session.Order("priority desc, run_at asc").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("coyote/repo: listing claimable jobs: %w", err)
	}
	return rows, nil
}

func (q *jobQueue) take(handle *gorm.DB, jobID, worker string, now time.Time) (bool, error) {
	result := handle.Model(&jobs.Record{}).
		Where("id = ? AND state = ?", jobID, jobs.Queued).
		Updates(map[string]any{
			"state":      jobs.Running,
			"locked_by":  worker,
			"locked_at":  now,
			"attempts":   gorm.Expr("attempts + 1"),
			"updated_at": now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("coyote/repo: claiming job %s: %w", jobID, result.Error)
	}
	return result.RowsAffected == 1, nil
}

func running(record jobs.Record, worker string, now time.Time) jobs.Record {
	record.State = jobs.Running
	record.LockedBy = worker
	record.LockedAt = &now
	record.Attempts++
	record.UpdatedAt = now
	return record
}

func (q *jobQueue) Heartbeat(ctx context.Context, jobID string, at time.Time) error {
	handle, err := q.handle(ctx)
	if err != nil {
		return err
	}
	result := handle.Model(&jobs.Record{}).
		Where("id = ? AND state = ?", jobID, jobs.Running).
		Updates(map[string]any{"locked_at": at.UTC(), "updated_at": at.UTC()})
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: extending the lock on job %s: %w", jobID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s is not running", jobs.ErrNotFound, jobID)
	}
	return nil
}

func (q *jobQueue) Recover(ctx context.Context, lockedBefore time.Time) (int, error) {
	handle, err := q.handle(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	abandoned := handle.Model(&jobs.Record{}).
		Where("state = ? AND locked_at IS NOT NULL AND locked_at < ?", jobs.Running, lockedBefore.UTC())

	buried, err := q.bury(handle, lockedBefore, now)
	if err != nil {
		return 0, err
	}

	result := abandoned.Updates(map[string]any{
		"state":      jobs.Queued,
		"locked_by":  "",
		"locked_at":  nil,
		"updated_at": now,
	})
	if result.Error != nil {
		return buried, fmt.Errorf("coyote/repo: recovering abandoned jobs: %w", result.Error)
	}
	return buried + int(result.RowsAffected), nil
}

func (q *jobQueue) bury(handle *gorm.DB, lockedBefore, now time.Time) (int, error) {
	result := handle.Model(&jobs.Record{}).
		Where("state = ? AND locked_at IS NOT NULL AND locked_at < ?", jobs.Running, lockedBefore.UTC()).
		Where("max_attempts <> ? AND attempts >= max_attempts", jobs.Forever).
		Updates(map[string]any{
			"state":       jobs.Dead,
			"locked_by":   "",
			"locked_at":   nil,
			"last_error":  "the worker holding this job stopped reporting and its attempts were used up",
			"finished_at": now,
			"updated_at":  now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("coyote/repo: burying exhausted jobs: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}
