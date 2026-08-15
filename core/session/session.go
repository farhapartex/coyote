package session

import (
	"sync"
	"time"
)

type status int

const (
	unmodified status = iota
	modified
	destroyed
)

const (
	flashKey = "_coyote_flashes"
	csrfKey  = "_coyote_csrf"
	userKey  = "_coyote_user"
)

type Session struct {
	mu      sync.RWMutex
	id      string
	oldID   string
	values  map[string]any
	created time.Time
	expires time.Time
	status  status
	fresh   bool
}

func newSession(id string, lifetime time.Duration) *Session {
	now := time.Now()
	return &Session{
		id:      id,
		values:  make(map[string]any),
		created: now,
		expires: now.Add(lifetime),
		status:  modified,
		fresh:   true,
	}
}

func Restore(id string, values map[string]any, created, expires time.Time) *Session {
	if values == nil {
		values = make(map[string]any)
	}
	if created.IsZero() {
		created = time.Now()
	}
	return &Session{
		id:      id,
		values:  values,
		created: created,
		expires: expires,
		status:  unmodified,
	}
}

func (s *Session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

func (s *Session) CreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.created
}

func (s *Session) ExpiresAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expires
}
