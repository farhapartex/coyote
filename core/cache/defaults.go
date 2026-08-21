package cache

import "time"

const (
	Forever            = time.Duration(-1)
	DefaultTTL         = 5 * time.Minute
	DefaultMaxEntries  = 10000
	DefaultCleanup     = 5 * time.Minute
	DefaultDialTimeout = 2 * time.Second
	DefaultPrefix      = "coyote"
	UnknownLength      = -1
)
