package auth

import (
	"errors"
	"sync"
	"time"
)

var ErrTooManyAttempts = errors.New("coyote/auth: too many failed sign-in attempts")

type ThrottlePolicy struct {
	Enabled     bool
	MaxAttempts int
	Window      time.Duration
	Lockout     time.Duration
}

func (p ThrottlePolicy) normalise() ThrottlePolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 5
	}
	if p.Window <= 0 {
		p.Window = 15 * time.Minute
	}
	if p.Lockout <= 0 {
		p.Lockout = p.Window
	}
	return p
}

type LoginLimiter interface {
	Allow(key string) bool
	Fail(key string)
	Reset(key string)
}

type attempts struct {
	count       int
	first       time.Time
	lockedUntil time.Time
}

type memoryLimiter struct {
	policy ThrottlePolicy
	mu     sync.Mutex
	seen   map[string]*attempts
}

func NewLoginLimiter(policy ThrottlePolicy) LoginLimiter {
	return &memoryLimiter{policy: policy.normalise(), seen: map[string]*attempts{}}
}

func (l *memoryLimiter) Allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	record, found := l.seen[key]
	if !found {
		return true
	}
	if now.Before(record.lockedUntil) {
		return false
	}
	if !record.lockedUntil.IsZero() {
		delete(l.seen, key)
	}
	return true
}

func (l *memoryLimiter) Fail(key string) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	record, found := l.seen[key]
	if !found || now.Sub(record.first) > l.policy.Window {
		l.seen[key] = &attempts{count: 1, first: now}
		return
	}
	record.count++
	if record.count >= l.policy.MaxAttempts {
		record.lockedUntil = now.Add(l.policy.Lockout)
	}
}

func (l *memoryLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.seen, key)
}

func (l *memoryLimiter) sweep(now time.Time) {
	if len(l.seen) < 128 {
		return
	}
	for key, record := range l.seen {
		expired := now.Sub(record.first) > l.policy.Window && now.After(record.lockedUntil)
		if expired {
			delete(l.seen, key)
		}
	}
}
