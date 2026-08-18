package migrate

import (
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu         sync.RWMutex
	migrations []Migration
	seen       map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{seen: make(map[string]bool)}
}

var defaultRegistry = NewRegistry()

func Register(migrations ...Migration) {
	if err := defaultRegistry.Add(migrations...); err != nil {
		panic(err)
	}
}

func Registered() []Migration { return defaultRegistry.All() }

func Default() *Registry { return defaultRegistry }

func Reset() { defaultRegistry.Clear() }

func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.migrations = nil
	r.seen = make(map[string]bool)
}

func (r *Registry) Add(migrations ...Migration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range migrations {
		if m.ID == "" {
			return fmt.Errorf("coyote/migrate: migration without an ID")
		}
		if r.seen[m.ID] {
			return fmt.Errorf("coyote/migrate: duplicate migration %q", m.ID)
		}
		r.seen[m.ID] = true
		r.migrations = append(r.migrations, m)
	}
	return nil
}

func (r *Registry) All() []Migration {
	r.mu.RLock()
	out := append([]Migration{}, r.migrations...)
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.migrations)
}
