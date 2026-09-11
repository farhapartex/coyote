package settings

import (
	"time"

	"github.com/farhapartex/coyote/core/jobs"
)

type Jobs struct {
	Enabled        bool
	Queue          jobs.Queue
	Queues         []string
	Workers        int
	MaxAttempts    int
	Backoff        time.Duration
	BackoffCeiling time.Duration
	ClaimTimeout   time.Duration
	DrainTimeout   time.Duration
	PollInterval   time.Duration
	DoneTTL        time.Duration
}

func (j Jobs) Active() bool { return j.Enabled }

func (j Jobs) RunsWorkers() bool { return j.Enabled && j.Workers > 0 }

func (j Jobs) QueueNames() []string {
	if len(j.Queues) == 0 {
		return []string{jobs.DefaultQueue}
	}
	return j.Queues
}
