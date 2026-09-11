package jobs

import (
	"math/rand/v2"
	"time"
)

const (
	DefaultBackoff = 10 * time.Second
	MaxBackoff     = 24 * time.Hour
	jitterFraction = 5
)

func Delay(base, ceiling time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = DefaultBackoff
	}
	cap := ceiling
	if cap <= 0 || cap > MaxBackoff {
		cap = MaxBackoff
	}
	if base > cap {
		base = cap
	}
	if attempt < 1 {
		attempt = 1
	}

	delay := base
	for range attempt - 1 {
		if delay > cap/2 {
			delay = cap
			break
		}
		delay *= 2
	}
	return delay + jitter(delay)
}

func jitter(delay time.Duration) time.Duration {
	span := int64(delay) / jitterFraction
	if span <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(span))
}
