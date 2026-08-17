package model

import (
	"reflect"
	"sort"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	models []Model
}

func NewRegistry(models ...Model) *Registry {
	return &Registry{models: models}
}

func (r *Registry) Add(models ...Model) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models = append(r.models, models...)
}

func (r *Registry) All() []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Model{}, r.models...)
}

func (r *Registry) Entities() []any {
	models := r.All()
	out := make([]any, 0, len(models))
	for _, m := range models {
		out = append(out, m.Entity)
	}
	return out
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

func (r *Registry) Aliases() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := map[string]bool{}
	out := []string{}
	for _, m := range r.models {
		if m.Alias == "" || seen[m.Alias] {
			continue
		}
		seen[m.Alias] = true
		out = append(out, m.Alias)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) For(alias string) []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := []Model{}
	for _, m := range r.models {
		if m.Alias == alias {
			out = append(out, m)
		}
	}
	return out
}

func (r *Registry) AliasOf(entity any) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target := reflect.TypeOf(entity)
	for _, m := range r.models {
		if reflect.TypeOf(m.Entity) == target {
			return m.Alias
		}
	}
	return ""
}
