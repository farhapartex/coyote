package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

const heartbeatDivisor = 3

func (r *Runner) work(worker string) {
	defer r.running.Done()

	for !r.stopping() {
		claimed, err := r.queue.Claim(r.ctx, worker, r.queues, 1)
		if err != nil {
			if r.ctx.Err() == nil {
				r.log.Error("jobs could not be claimed", slog.String("worker", worker), slog.Any("error", err))
			}
			if !r.wait(r.poll) {
				return
			}
			continue
		}
		if len(claimed) == 0 {
			if !r.wait(r.poll) {
				return
			}
			continue
		}
		r.execute(worker, claimed[0])
	}
}

func (r *Runner) execute(worker string, record Record) {
	started := time.Now()
	beating := r.beat(record.ID)
	err := r.dispatch(record)
	close(beating)

	book := context.WithoutCancel(r.ctx)
	took := time.Since(started).Round(time.Millisecond)

	if err == nil {
		if done := r.queue.Complete(book, record.ID, time.Now()); done != nil {
			r.log.Error("a finished job could not be recorded",
				slog.String("job", record.ID), slog.Any("error", done))
			return
		}
		r.log.Info("job done",
			slog.String("kind", record.Kind),
			slog.String("job", record.ID),
			slog.String("worker", worker),
			slog.Duration("took", took),
		)
		return
	}

	r.bury(book, record, worker, err, took)
}

func (r *Runner) bury(ctx context.Context, record Record, worker string, cause error, took time.Duration) {
	retryAt := r.retry(record, cause)
	attrs := []any{
		slog.String("kind", record.Kind),
		slog.String("job", record.ID),
		slog.String("worker", worker),
		slog.Int("attempt", record.Attempts),
		slog.Duration("took", took),
		slog.Any("error", cause),
	}
	if retryAt != nil {
		attrs = append(attrs, slog.Time("retry_at", *retryAt))
		r.log.Warn("job failed, retrying", attrs...)
	} else {
		r.log.Error("job failed and will not be retried", attrs...)
	}

	if err := r.queue.Fail(ctx, record.ID, cause.Error(), retryAt); err != nil {
		r.log.Error("a failed job could not be recorded",
			slog.String("job", record.ID), slog.Any("error", err))
	}
}

func (r *Runner) retry(record Record, cause error) *time.Time {
	if errors.Is(cause, ErrUnknownKind) || errors.Is(cause, ErrPayloadInvalid) {
		return nil
	}
	if record.Exhausted() {
		return nil
	}
	at := time.Now().Add(Delay(r.backoff, r.ceiling, record.Attempts))
	return &at
}

func (r *Runner) dispatch(record Record) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		r.log.Error("a job panicked",
			slog.String("kind", record.Kind),
			slog.String("job", record.ID),
			slog.Any("error", recovered),
			slog.String("stack", string(debug.Stack())),
		)
		err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
	}()
	return r.registry.Run(r.ctx, record.Kind, record.Payload)
}

func (r *Runner) beat(jobID string) chan struct{} {
	finished := make(chan struct{})
	every := r.claim / heartbeatDivisor
	if every <= 0 {
		close(finished)
		return finished
	}

	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-finished:
				return
			case <-ticker.C:
				if err := r.queue.Heartbeat(context.WithoutCancel(r.ctx), jobID, time.Now()); err != nil {
					return
				}
			}
		}
	}()
	return finished
}
