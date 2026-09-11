package jobs

import "time"

type Options struct {
	Queue       string
	Priority    int
	Delay       time.Duration
	MaxAttempts int
	Fingerprint string
}

func (o Options) job(kind string, payload []byte, now time.Time) Job {
	queue := o.Queue
	if queue == "" {
		queue = DefaultQueue
	}
	runAt := now
	if o.Delay > 0 {
		runAt = now.Add(o.Delay)
	}
	return Job{
		Kind:        kind,
		Queue:       queue,
		Payload:     payload,
		Priority:    o.Priority,
		RunAt:       runAt,
		MaxAttempts: o.MaxAttempts,
		Fingerprint: o.Fingerprint,
	}
}

func firstOption(opts []Options) Options {
	if len(opts) == 0 {
		return Options{}
	}
	return opts[0]
}
