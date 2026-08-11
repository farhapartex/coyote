package session

import (
	"crypto/rand"
	"encoding/base64"
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

type Flash struct {
	Kind    string
	Message string
}

type Session struct {
	mu      sync.RWMutex
	id      string
	oldID   string
	values  map[string]any
	created time.Time
	expires time.Time
	status  status
}

func newSession(id string, lifetime time.Duration) *Session {
	now := time.Now()
	return &Session{
		id:      id,
		values:  make(map[string]any),
		created: now,
		expires: now.Add(lifetime),
		status:  modified,
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

func (s *Session) Values() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out
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

func (s *Session) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key]
}

func (s *Session) GetString(key string) string {
	v, _ := s.Get(key).(string)
	return v
}

func (s *Session) GetInt(key string) int {
	v, _ := s.Get(key).(int)
	return v
}

func (s *Session) GetBool(key string) bool {
	v, _ := s.Get(key).(bool)
	return v
}

func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	s.status = modified
}

func (s *Session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	s.status = modified
}

func (s *Session) Pop(key string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[key]
	if ok {
		delete(s.values, key)
		s.status = modified
	}
	return v
}

func (s *Session) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.values))
	for k := range s.values {
		if k == flashKey || k == csrfKey {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = make(map[string]any)
	s.status = modified
}

func (s *Session) AddFlash(kind, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue, _ := s.values[flashKey].([]Flash)
	s.values[flashKey] = append(queue, Flash{Kind: kind, Message: message})
	s.status = modified
}

func (s *Session) Flashes() []Flash {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue, _ := s.values[flashKey].([]Flash)
	if len(queue) == 0 {
		return nil
	}
	delete(s.values, flashKey)
	s.status = modified
	return queue
}

func (s *Session) UserID() string {
	return s.GetString(userKey)
}

func (s *Session) SetUserID(id string) {
	s.Set(userKey, id)
}

func (s *Session) ClearUser() {
	s.Delete(userKey)
}

func (s *Session) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = make(map[string]any)
	s.status = destroyed
}

func (s *Session) Destroyed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == destroyed
}

func (s *Session) Modified() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == modified
}

func (s *Session) Expired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Now().After(s.expires)
}

func (s *Session) touch(lifetime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expires = time.Now().Add(lifetime)
	s.status = modified
}

func (s *Session) renew(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oldID, s.id = s.id, id
	s.status = modified
}

func (s *Session) takeOldID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.oldID
	s.oldID = ""
	return old
}

func newID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
