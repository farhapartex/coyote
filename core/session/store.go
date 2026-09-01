package session

import (
	"context"
	"sort"
	"sync"
	"time"
)

type Store interface {
	Load(ctx context.Context, id string) (*Session, bool)
	Save(ctx context.Context, s *Session) error
	Delete(ctx context.Context, id string) error
}

type ManageableStore interface {
	Store
	Count(ctx context.Context) (int, error)
	All(ctx context.Context) ([]*Session, error)
	DeleteByUserID(ctx context.Context, userID string) (int, error)
}

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stop     chan struct{}
	stopOnce sync.Once
}

func NewMemoryStore(gcInterval time.Duration) *MemoryStore {
	m := &MemoryStore{
		sessions: make(map[string]*Session),
		stop:     make(chan struct{}),
	}
	if gcInterval > 0 {
		go m.collect(gcInterval)
	}
	return m
}

func (m *MemoryStore) Load(ctx context.Context, id string) (*Session, bool) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if s.Expired() {
		_ = m.Delete(ctx, id)
		return nil, false
	}
	return s, true
}

func (m *MemoryStore) Save(_ context.Context, s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID()] = s
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *MemoryStore) Count(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions), nil
}

func (m *MemoryStore) All(_ context.Context) ([]*Session, error) {
	m.mu.RLock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt().After(out[j].CreatedAt())
	})
	return out, nil
}

func (m *MemoryStore) DeleteByUserID(_ context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, s := range m.sessions {
		if s.UserID() == userID {
			delete(m.sessions, id)
			n++
		}
	}
	return n, nil
}

func (m *MemoryStore) Close() {
	m.stopOnce.Do(func() { close(m.stop) })
}

func (m *MemoryStore) collect(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			for id, s := range m.sessions {
				if s.Expired() {
					delete(m.sessions, id)
				}
			}
			m.mu.Unlock()
		}
	}
}
