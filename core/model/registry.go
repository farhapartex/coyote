package model

import "sync"

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
