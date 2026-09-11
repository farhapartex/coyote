package jobs

import "time"

const (
	DefaultQueue       = "default"
	DefaultMaxAttempts = 3
	Forever            = -1
)

type Job struct {
	Kind        string
	Queue       string
	Payload     []byte
	Priority    int
	RunAt       time.Time
	MaxAttempts int
	Fingerprint string
}

func (j Job) Valid() error {
	if j.Kind == "" {
		return ErrKindMissing
	}
	return nil
}
