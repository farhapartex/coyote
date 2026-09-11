package jobs

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultPollInterval = time.Second
	DefaultClaimTimeout = 5 * time.Minute
	DefaultDrainTimeout = 30 * time.Second
	cancelGrace         = 5 * time.Second
)

type Runner struct {
	queue     Queue
	registry  *Registry
	schedules *Schedule
	slots     map[string]time.Time
	slotMu    sync.Mutex
	name      string
	queues    []string
	workers   int
	poll      time.Duration
	claim     time.Duration
	drain     time.Duration
	backoff   time.Duration
	ceiling   time.Duration
	doneTTL   time.Duration
	log       *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	stop      chan struct{}
	running   sync.WaitGroup
	mu        sync.Mutex
	started   bool
	closed    bool
}

func (r *Runner) Start() {
	if r.queue == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.closed {
		return
	}
	r.started = true

	r.log.Info("jobs starting",
		slog.Int("workers", r.workers),
		slog.Any("queues", r.queues),
	)
	for i := range r.workers {
		r.running.Add(1)
		go r.work(workerName(r.name, i))
	}
	r.running.Add(1)
	go r.maintain()
	r.running.Add(1)
	go r.tick()
}

func (r *Runner) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	started := r.started
	r.mu.Unlock()

	close(r.stop)
	if !started {
		r.cancel()
		return nil
	}
	r.awaitDrain()
	r.cancel()
	return nil
}

func (r *Runner) awaitDrain() {
	finished := make(chan struct{})
	go func() {
		r.running.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		return
	case <-time.After(r.drain):
	}

	r.log.Warn("jobs did not finish draining, cancelling what is still running",
		slog.Duration("waited", r.drain))
	r.cancel()

	select {
	case <-finished:
	case <-time.After(cancelGrace):
		r.log.Error("a job ignored cancellation; it will be reclaimed after the claim timeout",
			slog.Duration("claim_timeout", r.claim))
	}
}

func (r *Runner) stopping() bool {
	select {
	case <-r.stop:
		return true
	default:
		return false
	}
}

func (r *Runner) wait(every time.Duration) bool {
	select {
	case <-r.stop:
		return false
	case <-r.ctx.Done():
		return false
	case <-time.After(every):
		return true
	}
}

func workerName(prefix string, index int) string {
	return prefix + "-" + strconv.Itoa(index)
}
