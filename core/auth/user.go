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
	ErrUserNotFound   = errors.New("coyote/auth: user not found")
	ErrUserExists     = errors.New("coyote/auth: username already taken")
	ErrEmailExists    = errors.New("coyote/auth: email already registered")
	ErrInvalidUser    = errors.New("coyote/auth: invalid username")
	ErrInvalidEmail   = errors.New("coyote/auth: invalid email address")
	ErrLastSuperadmin = errors.New("coyote/auth: cannot remove the last superadmin")
)

var (
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._@+-]{3,64}$`)
	emailPattern    = regexp.MustCompile(`^[^@\s]+@[^@\s.]+\.[^@\s]+$`)
)

type User struct {
	ID           string    `db:"id"`
	FirstName    string    `db:"first_name"`
	LastName     string    `db:"last_name"`
	Email        string    `db:"email"`
	Username     string    `db:"username"`
	Password     string    `db:"password"`
	IsActive     bool      `db:"is_active"`
	IsSuperadmin bool      `db:"is_superadmin"`
	LastLoginAt  time.Time `db:"last_login_at"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (u *User) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
}

func (u *User) DisplayName() string {
	if name := u.FullName(); name != "" {
		return name
	}
	return u.Username
}

func (u *User) Initials() string {
	first, last := strings.TrimSpace(u.FirstName), strings.TrimSpace(u.LastName)
	switch {
	case first != "" && last != "":
		return strings.ToUpper(first[:1] + last[:1])
	case first != "":
		return strings.ToUpper(first[:1])
	case last != "":
		return strings.ToUpper(last[:1])
	}
	if username := strings.TrimSpace(u.Username); username != "" {
		return strings.ToUpper(username[:1])
	}
	return "?"
}

func (u *User) HasUsablePassword() bool {
	return LooksHashed(u.Password)
}

func (u *User) HasLoggedIn() bool {
	return !u.LastLoginAt.IsZero()
}

func (u *User) Clone() *User {
	copied := *u
	return &copied
}

func (u *User) Validate() error {
	if !usernamePattern.MatchString(u.Username) {
		return ErrInvalidUser
	}
	if u.Email != "" && !emailPattern.MatchString(u.Email) {
		return ErrInvalidEmail
	}
	return nil
}

type Store interface {
	ByID(id string) (*User, error)
	ByUsername(username string) (*User, error)
	ByEmail(email string) (*User, error)
	Create(u *User) error
	Update(u *User) error
	Delete(id string) error
	All() []*User
	Count() int
}

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

	now := time.Now()
	if u.ID == "" {
		u.ID = newUserID()
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now

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
	if existing.IsSuperadmin && existing.IsActive &&
		(!u.IsSuperadmin || !u.IsActive) && m.countSuperadmins() < 2 {
		return ErrLastSuperadmin
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
	if u.IsSuperadmin && u.IsActive && m.countSuperadmins() < 2 {
		return ErrLastSuperadmin
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

func (m *MemoryStore) countSuperadmins() int {
	n := 0
	for _, u := range m.users {
		if u.IsSuperadmin && u.IsActive {
			n++
		}
	}
	return n
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func newUserID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
