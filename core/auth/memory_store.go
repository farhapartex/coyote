package auth

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/farhapartex/coyote/lib/text"
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

func (m *MemoryStore) ByID(_ context.Context, id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u.Clone(), nil
}

func (m *MemoryStore) ByUsername(_ context.Context, username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lookup(m.byName, username)
}

func (m *MemoryStore) ByEmail(_ context.Context, email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lookup(m.byEmail, email)
}

func (m *MemoryStore) lookup(index map[string]string, key string) (*User, error) {
	id, ok := index[text.Fold(key)]
	if !ok {
		return nil, ErrUserNotFound
	}
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u.Clone(), nil
}

func (m *MemoryStore) Create(_ context.Context, u *User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	name := text.Fold(u.Username)
	if _, exists := m.byName[name]; exists {
		return ErrUserExists
	}
	email := text.Fold(u.Email)
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

func (m *MemoryStore) Update(_ context.Context, u *User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.users[u.ID]
	if !ok {
		return ErrUserNotFound
	}

	name := text.Fold(u.Username)
	if id, taken := m.byName[name]; taken && id != u.ID {
		return ErrUserExists
	}
	email := text.Fold(u.Email)
	if email != "" {
		if id, taken := m.byEmail[email]; taken && id != u.ID {
			return ErrEmailExists
		}
	}
	delete(m.byName, text.Fold(existing.Username))
	delete(m.byEmail, text.Fold(existing.Email))
	m.byName[name] = u.ID
	if email != "" {
		m.byEmail[email] = u.ID
	}

	u.CreatedAt = existing.CreatedAt
	u.UpdatedAt = time.Now()
	m.users[u.ID] = u.Clone()
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrUserNotFound
	}
	delete(m.byName, text.Fold(u.Username))
	delete(m.byEmail, text.Fold(u.Email))
	delete(m.users, id)
	return nil
}

func (m *MemoryStore) All(_ context.Context) ([]*User, error) {
	m.mu.RLock()
	out := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u.Clone())
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username)
	})
	return out, nil
}

func (m *MemoryStore) Search(ctx context.Context, term string, limit, offset int) ([]*User, int, error) {
	everyone, err := m.All(ctx)
	if err != nil {
		return nil, 0, err
	}
	matched := make([]*User, 0, len(everyone))
	for _, u := range everyone {
		if Matches(u, term) {
			matched = append(matched, u)
		}
	}
	return Window(matched, limit, offset), len(matched), nil
}

func (m *MemoryStore) Recent(ctx context.Context, n int) ([]*User, error) {
	everyone, err := m.All(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(everyone, func(i, j int) bool {
		return everyone[i].CreatedAt.After(everyone[j].CreatedAt)
	})
	return Window(everyone, n, 0), nil
}

func (m *MemoryStore) Stats(ctx context.Context) (Stats, error) {
	everyone, err := m.All(ctx)
	if err != nil {
		return Stats{}, err
	}
	out := Stats{Total: len(everyone)}
	for _, u := range everyone {
		switch {
		case u.IsSuperadmin:
			out.Superadmins++
		case u.IsStaff:
			out.Staff++
		}
	}
	return out, nil
}

func (m *MemoryStore) Count(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users), nil
}

func (m *MemoryStore) CountActiveSuperadmins(_ context.Context) (int, error) {
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
