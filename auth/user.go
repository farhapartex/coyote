package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrUserNotFound  = errors.New("coyote/auth: user not found")
	ErrUserExists    = errors.New("coyote/auth: username already taken")
	ErrInvalidUser   = errors.New("coyote/auth: invalid username")
	ErrLastSuperuser = errors.New("coyote/auth: cannot remove the last superuser")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._@+-]{3,64}$`)

type User struct {
	ID           string
	Username     string
	Email        string
	FullName     string
	PasswordHash string
	IsActive     bool
	IsStaff      bool
	IsSuperuser  bool
	CreatedAt    time.Time
	LastLogin    time.Time
}

func (u *User) DisplayName() string {
	if u.FullName != "" {
		return u.FullName
	}
	return u.Username
}

func (u *User) Initials() string {
	name := strings.TrimSpace(u.DisplayName())
	if name == "" {
		return "?"
	}
	fields := strings.Fields(name)
	if len(fields) == 1 {
		return strings.ToUpper(fields[0][:1])
	}
	return strings.ToUpper(fields[0][:1] + fields[len(fields)-1][:1])
}

func (u *User) clone() *User {
	copied := *u
	return &copied
}

type Store interface {
	ByID(id string) (*User, error)
	ByUsername(username string) (*User, error)
	Create(u *User) error
	Update(u *User) error
	Delete(id string) error
	All() []*User
	Count() int
}

type MemoryStore struct {
	mu     sync.RWMutex
	users  map[string]*User
	byName map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:  make(map[string]*User),
		byName: make(map[string]string),
	}
}

func (m *MemoryStore) ByID(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u.clone(), nil
}

func (m *MemoryStore) ByUsername(username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byName[normalize(username)]
	if !ok {
		return nil, ErrUserNotFound
	}
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u.clone(), nil
}

func (m *MemoryStore) Create(u *User) error {
	if !usernamePattern.MatchString(u.Username) {
		return ErrInvalidUser
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := normalize(u.Username)
	if _, exists := m.byName[key]; exists {
		return ErrUserExists
	}
	if u.ID == "" {
		u.ID = newUserID()
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	m.users[u.ID] = u.clone()
	m.byName[key] = u.ID
	return nil
}

func (m *MemoryStore) Update(u *User) error {
	if !usernamePattern.MatchString(u.Username) {
		return ErrInvalidUser
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.users[u.ID]
	if !ok {
		return ErrUserNotFound
	}
	key := normalize(u.Username)
	if id, taken := m.byName[key]; taken && id != u.ID {
		return ErrUserExists
	}
	delete(m.byName, normalize(existing.Username))
	m.byName[key] = u.ID
	m.users[u.ID] = u.clone()
	return nil
}

func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrUserNotFound
	}
	if u.IsSuperuser && m.countSuperusers() < 2 {
		return ErrLastSuperuser
	}
	delete(m.byName, normalize(u.Username))
	delete(m.users, id)
	return nil
}

func (m *MemoryStore) All() []*User {
	m.mu.RLock()
	out := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u.clone())
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

func (m *MemoryStore) countSuperusers() int {
	n := 0
	for _, u := range m.users {
		if u.IsSuperuser && u.IsActive {
			n++
		}
	}
	return n
}

func normalize(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func newUserID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
