package settings

import "time"

type RateLimit struct {
	Requests   int
	Window     time.Duration
	Burst      int
	TrustProxy bool
}

func (r RateLimit) Enabled() bool { return r.Requests > 0 && r.Window > 0 }

func (r RateLimit) Capacity() int {
	if r.Burst > 0 {
		return r.Burst
	}
	return r.Requests
}
