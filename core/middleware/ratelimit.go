package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/farhapartex/coyote/core/settings"
)

type KeyFunc func(*http.Request) string

func RateLimit(policy settings.RateLimit) Middleware {
	return RateLimitBy(policy, ClientIP(policy.TrustProxy))
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

func ClientIP(trustProxy bool) KeyFunc {
	return func(r *http.Request) string {
		if trustProxy {
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				return strings.TrimSpace(strings.Split(forwarded, ",")[0])
			}
			if real := r.Header.Get("X-Real-Ip"); real != "" {
				return strings.TrimSpace(real)
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}

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
	stale := s.window * 2
	for key, entry := range s.buckets {
		if now.Sub(entry.seen) > stale {
			delete(s.buckets, key)
		}
	}
}
