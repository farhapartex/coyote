package settings

import "time"

type PageCache struct {
	Enabled bool
	Alias   string
	TTL     time.Duration
	Paths   []string
	Skip    []string
}

func (p PageCache) Active() bool { return p.Enabled && p.TTL > 0 }
