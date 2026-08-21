package cache

import "sync/atomic"

type counters struct {
	hits    atomic.Int64
	misses  atomic.Int64
	sets    atomic.Int64
	deletes atomic.Int64
	errors  atomic.Int64
}

func (c *counters) snapshot() Stats {
	return Stats{
		Hits:    c.hits.Load(),
		Misses:  c.misses.Load(),
		Sets:    c.sets.Load(),
		Deletes: c.deletes.Load(),
		Errors:  c.errors.Load(),
	}
}
