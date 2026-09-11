package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/farhapartex/coyote/lib/id"
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
