package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/lib/clientip"
)

type KeyFunc func(*http.Request) string

func RateLimit(policy settings.RateLimit, trustedProxies int) Middleware {
	return RateLimitBy(policy, ClientIP(trustedProxies))
}

func RateLimitBy(policy settings.RateLimit, key KeyFunc) Middleware {
	buckets := newBucketSet(policy)
	retryAfter := strconv.Itoa(int(policy.Window.Seconds()))
	if retryAfter == "0" {
		retryAfter = "1"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remaining, allowed := buckets.take(key(r))
			header := w.Header()
			header.Set("RateLimit-Limit", strconv.Itoa(policy.Capacity()))
			header.Set("RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				header.Set("Retry-After", retryAfter)
				http.Error(w, "429 too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(trustedProxies int) KeyFunc {
	return func(r *http.Request) string { return clientip.Key(r, trustedProxies) }
}

const (
	maxTrackedClients = 100_000
	evictionSample    = 8
)

type bucket struct {
	tokens float64
	seen   time.Time
}

type bucketSet struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity float64
	refill   float64
	window   time.Duration
	swept    time.Time
}

func newBucketSet(policy settings.RateLimit) *bucketSet {
	return &bucketSet{
		buckets:  make(map[string]*bucket),
		capacity: float64(policy.Capacity()),
		refill:   float64(policy.Requests) / policy.Window.Seconds(),
		window:   policy.Window,
		swept:    time.Now(),
	}
}

func (s *bucketSet) take(key string) (int, bool) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweep(now)

	entry, found := s.buckets[key]
	if !found {
		s.makeRoom()
		entry = &bucket{tokens: s.capacity, seen: now}
		s.buckets[key] = entry
	} else {
		entry.tokens += now.Sub(entry.seen).Seconds() * s.refill
		if entry.tokens > s.capacity {
			entry.tokens = s.capacity
		}
		entry.seen = now
	}

	if entry.tokens < 1 {
		return 0, false
	}
	entry.tokens--
	return int(entry.tokens), true
}

func (s *bucketSet) sweep(now time.Time) {
	if now.Sub(s.swept) < s.window {
		return
	}
	s.swept = now
	s.expire(now)
}

func (s *bucketSet) expire(now time.Time) {
	stale := s.window * 2
	for key, entry := range s.buckets {
		if now.Sub(entry.seen) > stale {
			delete(s.buckets, key)
		}
	}
}

func (s *bucketSet) makeRoom() {
	for len(s.buckets) >= maxTrackedClients {
		s.evictOldest()
	}
}

func (s *bucketSet) evictOldest() {
	oldest, oldestSeen := "", time.Time{}
	sampled := 0
	for key, entry := range s.buckets {
		if sampled == evictionSample {
			break
		}
		sampled++
		if oldestSeen.IsZero() || entry.seen.Before(oldestSeen) {
			oldest, oldestSeen = key, entry.seen
		}
	}
	delete(s.buckets, oldest)
}
