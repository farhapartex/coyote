package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Handler func(ctx context.Context, payload []byte) error

type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	parent   *Registry
}

func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
}

func RegistryUnder(parent *Registry) *Registry {
	return &Registry{handlers: map[string]Handler{}, parent: parent}
}

func (r *Registry) Add(kind string, handler Handler) error {
	if kind == "" {
		return ErrKindMissing
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.handlers[kind]; taken {
		return fmt.Errorf("%w: %s", ErrDuplicateKind, kind)
	}
	if r.parent != nil && r.parent.Knows(kind) {
		return fmt.Errorf("%w: %s", ErrDuplicateKind, kind)
	}
	r.handlers[kind] = handler
	return nil
}

func (r *Registry) Lookup(kind string) (Handler, bool) {
	r.mu.RLock()
	handler, found := r.handlers[kind]
	r.mu.RUnlock()
	if found {
		return handler, true
	}
	if r.parent != nil {
		return r.parent.Lookup(kind)
	}
	return nil, false
}

func (r *Registry) Knows(kind string) bool {
	_, found := r.Lookup(kind)
	return found
}

func (r *Registry) Kinds() []string {
	seen := map[string]bool{}
	if r.parent != nil {
		for _, kind := range r.parent.Kinds() {
			seen[kind] = true
		}
	}
	r.mu.RLock()
	for kind := range r.handlers {
		seen[kind] = true
	}
	r.mu.RUnlock()

	out := make([]string, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) Run(ctx context.Context, kind string, payload []byte) error {
	handler, found := r.Lookup(kind)
	if !found {
		return fmt.Errorf("%w: %s", ErrUnknownKind, kind)
	}
	return handler(ctx, payload)
}
