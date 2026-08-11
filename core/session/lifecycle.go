package session

import "time"

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
