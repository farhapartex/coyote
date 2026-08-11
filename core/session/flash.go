package session

type Flash struct {
	Kind    string
	Message string
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
