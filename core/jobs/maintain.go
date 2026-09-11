package jobs

import (
	"context"
	"log/slog"
	"time"
)

func (r *Runner) maintain() {
	defer r.running.Done()

	for {
		if !r.wait(r.claim) {
			return
		}
		r.reclaim()
		r.tidy()
	}
}

func (r *Runner) reclaim() {
	recovered, err := r.queue.Recover(r.ctx, time.Now().Add(-r.claim))
	if err != nil {
		if r.ctx.Err() == nil {
			r.log.Error("abandoned jobs could not be recovered", slog.Any("error", err))
		}
		return
	}
	if recovered > 0 {
		r.log.Warn("jobs were reclaimed from a worker that stopped reporting",
			slog.Int("jobs", recovered),
			slog.Duration("claim_timeout", r.claim),
		)
	}
}

func (r *Runner) tidy() {
	if r.doneTTL <= 0 {
		return
	}
	removed, err := r.queue.Sweep(r.ctx, time.Now().Add(-r.doneTTL))
	if err != nil {
		if r.ctx.Err() == nil {
			r.log.Error("finished jobs could not be swept", slog.Any("error", err))
		}
		return
	}
	if removed > 0 {
		r.log.Debug("finished jobs swept", slog.Int("jobs", removed))
	}
}

func (r *Runner) Recover(ctx context.Context) (int, error) {
	return r.queue.Recover(ctx, time.Now().Add(-r.claim))
}
