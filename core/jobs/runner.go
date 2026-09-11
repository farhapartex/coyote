package jobs

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/farhapartex/coyote/lib/id"
)

const (
	DefaultPollInterval = time.Second
	DefaultClaimTimeout = 5 * time.Minute
	DefaultDrainTimeout = 30 * time.Second
	cancelGrace         = 5 * time.Second
)

type RunnerOptions struct {
	Queue        Queue
	Registry     *Registry
	Schedule     *Schedule
	Name         string
	Queues       []string
	Workers      int
	PollInterval time.Duration
	ClaimTimeout time.Duration
	DrainTimeout time.Duration
	Backoff      time.Duration
	Ceiling      time.Duration
	DoneTTL      time.Duration
	Logger       *slog.Logger
}

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
	stopOnce  sync.Once
	running   sync.WaitGroup
	started   sync.Once
}

func NewRunner(opts RunnerOptions) *Runner {
	if opts.Registry == nil {
		opts.Registry = registry
	}
	if opts.Schedule == nil {
		opts.Schedule = schedules
	}
	if opts.Name == "" {
		opts.Name = "worker-" + id.MustShort()
	}
	if len(opts.Queues) == 0 {
		opts.Queues = []string{DefaultQueue}
	}
	if opts.Workers < 1 {
		opts.Workers = 1
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}
	if opts.ClaimTimeout <= 0 {
		opts.ClaimTimeout = DefaultClaimTimeout
	}
	if opts.DrainTimeout <= 0 {
		opts.DrainTimeout = DefaultDrainTimeout
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		queue:     opts.Queue,
		registry:  opts.Registry,
		schedules: opts.Schedule,
		slots:     map[string]time.Time{},
		name:      opts.Name,
		queues:    opts.Queues,
		workers:   opts.Workers,
		poll:      opts.PollInterval,
		claim:     opts.ClaimTimeout,
		drain:     opts.DrainTimeout,
		backoff:   opts.Backoff,
		ceiling:   opts.Ceiling,
		doneTTL:   opts.DoneTTL,
		log:       opts.Logger,
		ctx:       ctx,
		cancel:    cancel,
		stop:      make(chan struct{}),
	}
}

func (r *Runner) Start() {
	if r.queue == nil {
		return
	}
	r.started.Do(func() {
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
	})
}

func (r *Runner) Close() error {
	r.stopOnce.Do(func() {
		close(r.stop)

		finished := make(chan struct{})
		go func() {
			r.running.Wait()
			close(finished)
		}()

		select {
		case <-finished:
		case <-time.After(r.drain):
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
		r.cancel()
	})
	return nil
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
