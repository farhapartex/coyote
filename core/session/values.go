package session

func (s *Session) Values() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out
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
