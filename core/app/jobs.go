package app

import (
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/store"
)

func jobQueue(s Settings, a *App) jobs.Queue {
	if !s.Jobs.Enabled {
		return nil
	}
	if s.Jobs.Queue != nil {
		return s.Jobs.Queue
	}
	if s.Database().Engine == "" {
		return nil
	}
	return store.LazyJobs(a.DB, store.JobOptions{MaxAttempts: s.Jobs.MaxAttempts})
}

func (a *App) Queue() jobs.Queue { return a.queue }

func (a *App) Jobs() *jobs.Runner { return a.jobs }

func jobRunner(s Settings, a *App, queue jobs.Queue) *jobs.Runner {
	if queue == nil || !s.Jobs.RunsWorkers() {
		return nil
	}
	return a.newRunner(queue, s.Jobs.Workers, s.Jobs.QueueNames())
}

func (a *App) Worker(workers int, queues []string) (*jobs.Runner, error) {
	if a.queue == nil {
		return nil, jobs.ErrNoQueue
	}
	if workers < 1 {
		workers = a.Settings.Jobs.Workers
	}
	if workers < 1 {
		workers = 1
	}
	if len(queues) == 0 {
		queues = a.Settings.Jobs.QueueNames()
	}
	return a.newRunner(a.queue, workers, queues), nil
}

func (a *App) newRunner(queue jobs.Queue, workers int, queues []string) *jobs.Runner {
	s := a.Settings
	handlers := a.builtinHandlers()
	return jobs.NewRunner(jobs.RunnerOptions{
		Queue:        queue,
		Registry:     handlers,
		Schedule:     a.builtinSchedule(handlers),
		Queues:       queues,
		Workers:      workers,
		PollInterval: s.Jobs.PollInterval,
		ClaimTimeout: s.Jobs.ClaimTimeout,
		DrainTimeout: s.Jobs.DrainTimeout,
		Backoff:      s.Jobs.Backoff,
		Ceiling:      s.Jobs.BackoffCeiling,
		DoneTTL:      s.Jobs.DoneTTL,
		Logger:       a.Logger,
	})
}

func (a *App) startJobs() {
	if a.jobs != nil {
		a.jobs.Start()
	}
}

func (a *App) drainJobs() {
	if a.jobs != nil {
		_ = a.jobs.Close()
	}
}
