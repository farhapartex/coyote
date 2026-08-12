package auth

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	users   map[string]*User
	byName  map[string]string
	byEmail map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:   make(map[string]*User),
		byName:  make(map[string]string),
		byEmail: make(map[string]string),
	}
}

func (m *MemoryStore) ByID(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u.Clone(), nil
}

func (m *MemoryStore) ByUsername(username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lookup(m.byName, username)
}

func (m *MemoryStore) ByEmail(email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lookup(m.byEmail, email)
}

func (m *MemoryStore) lookup(index map[string]string, key string) (*User, error) {
	id, ok := index[normalize(key)]
	if !ok {
		return nil, ErrUserNotFound
	}
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u.Clone(), nil
}

func (m *MemoryStore) Create(u *User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	name := normalize(u.Username)
	if _, exists := m.byName[name]; exists {
		return ErrUserExists
	}
	email := normalize(u.Email)
	if email != "" {
		if _, exists := m.byEmail[email]; exists {
			return ErrEmailExists
		}
	}

	Prepare(u)

	m.users[u.ID] = u.Clone()
	m.byName[name] = u.ID
	if email != "" {
		m.byEmail[email] = u.ID
	}
	return nil
}

func (m *MemoryStore) Update(u *User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.users[u.ID]
	if !ok {
		return ErrUserNotFound
	}

	name := normalize(u.Username)
	if id, taken := m.byName[name]; taken && id != u.ID {
		return ErrUserExists
	}
	email := normalize(u.Email)
	if email != "" {
		if id, taken := m.byEmail[email]; taken && id != u.ID {
			return ErrEmailExists
		}
	}
	delete(m.byName, normalize(existing.Username))
	delete(m.byEmail, normalize(existing.Email))
	m.byName[name] = u.ID
	if email != "" {
		m.byEmail[email] = u.ID
	}

	u.CreatedAt = existing.CreatedAt
	u.UpdatedAt = time.Now()
	m.users[u.ID] = u.Clone()
	return nil
}

func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrUserNotFound
	}
	delete(m.byName, normalize(u.Username))
	delete(m.byEmail, normalize(u.Email))
	delete(m.users, id)
	return nil
}

func (m *MemoryStore) All() []*User {
	m.mu.RLock()
	out := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u.Clone())
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username)
	})
	return out
}

func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users)
}

func (m *MemoryStore) CountActiveSuperadmins() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, u := range m.users {
		if u.IsSuperadmin && u.IsActive {
			n++
		}
	}
	return n, nil
}
