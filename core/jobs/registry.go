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
}

func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
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
	r.handlers[kind] = handler
	return nil
}

func (r *Registry) Lookup(kind string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, found := r.handlers[kind]
	return handler, found
}

func (r *Registry) Knows(kind string) bool {
	_, found := r.Lookup(kind)
	return found
}

func (r *Registry) Kinds() []string {
	r.mu.RLock()
	out := make([]string, 0, len(r.handlers))
	for kind := range r.handlers {
		out = append(out, kind)
	}
	r.mu.RUnlock()
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
