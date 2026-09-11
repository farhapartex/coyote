package settings

import (
	"strconv"
	"strings"

	"github.com/farhapartex/coyote/core/jobs"
)

const minPollInterval = 100

func (s Settings) validateJobs(add func(string)) {
	queue := s.Jobs
	if !queue.Enabled {
		s.validateUnusedJobFields(add)
		return
	}

	if queue.Queue == nil && s.Database().Engine == "" {
		add("Jobs.Enabled is set but no database is configured; the queue is a table, so configure Databases or supply Jobs.Queue")
	}
	if len(queue.Queues) == 0 {
		add("Jobs.Queues is empty; list the queues a worker takes from, normally just " + strconv.Quote(jobs.DefaultQueue))
	}
	for i, name := range queue.Queues {
		if strings.TrimSpace(name) == "" {
			add("Jobs.Queues[" + strconv.Itoa(i) + "] is empty")
		}
	}
	if queue.Workers < 0 {
		add("Jobs.Workers cannot be negative; use 0 to run no workers inside the web process")
	}

	s.validateJobAttempts(add)
	s.validateJobTimings(add)

	if queue.DoneTTL < 0 {
		add("Jobs.DoneTTL cannot be negative; use 0 to keep finished jobs forever")
	}
}

func (s Settings) validateJobAttempts(add func(string)) {
	queue := s.Jobs

	if queue.MaxAttempts == 0 {
		add("Jobs.MaxAttempts is 0, which would bury every job on its first failure; set a count, or jobs.Forever to retry without a ceiling")
	}
	if queue.MaxAttempts < 0 && queue.MaxAttempts != jobs.Forever {
		add("Jobs.MaxAttempts " + strconv.Itoa(queue.MaxAttempts) + " is not a count; use a positive number or jobs.Forever")
	}
	if queue.Backoff <= 0 {
		add("Jobs.Backoff must be greater than zero; it is how long the first retry waits")
	}
	if queue.BackoffCeiling > 0 && queue.BackoffCeiling < queue.Backoff {
		add("Jobs.BackoffCeiling is shorter than Jobs.Backoff, so the very first retry would already be capped")
	}
}

func (s Settings) validateJobTimings(add func(string)) {
	queue := s.Jobs

	if queue.ClaimTimeout <= 0 {
		add("Jobs.ClaimTimeout must be greater than zero; it is how long a job may hold its lock without reporting before another worker reclaims it")
	}
	if queue.DrainTimeout <= 0 {
		add("Jobs.DrainTimeout must be greater than zero; it is how long a worker gets to finish its jobs on shutdown")
	}
	if queue.ClaimTimeout > 0 && queue.DrainTimeout > 0 && queue.ClaimTimeout <= queue.DrainTimeout {
		add("Jobs.ClaimTimeout must be longer than Jobs.DrainTimeout, or a job still finishing during shutdown is reclaimed and run twice")
	}
	if queue.PollInterval <= 0 {
		add("Jobs.PollInterval must be greater than zero; it is how long a worker waits before looking for work again")
	}
	if queue.PollInterval > 0 && queue.PollInterval.Milliseconds() < minPollInterval {
		add("Jobs.PollInterval below " + strconv.Itoa(minPollInterval) + "ms is a busy loop against the database")
	}
}

func (s Settings) validateUnusedJobFields(add func(string)) {
	if s.Jobs.Workers > 0 {
		add("Jobs.Workers is set but Jobs.Enabled is false, so no worker would run")
	}
	if s.Jobs.Queue != nil {
		add("Jobs.Queue is set but Jobs.Enabled is false, so the queue would never be read")
	}
}
