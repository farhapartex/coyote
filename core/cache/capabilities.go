package cache

import (
	"context"
	"time"
)

type Namespacer interface {
	Cache
	ClearPrefix(ctx context.Context, prefix string) error
}

type Counter interface {
	Cache
	Incr(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
}

type Multi interface {
	Cache
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
}

type Pinger interface {
	Cache
	Ping(ctx context.Context) error
}

type Measured interface {
	Cache
	Len() int
}

type Evicter interface {
	Cache
	Evictions() int64
}

type Closer interface {
	Close() error
}
