package cache

import (
	"context"
	"strings"
	"time"
)

type Options struct {
	Alias   string
	Backend string
	Prefix  string
	Version string
	TTL     time.Duration
	Codec   Codec
}

type Handle struct {
	store   Cache
	alias   string
	backend string
	prefix  string
	version string
	ttl     time.Duration
	codec   Codec
	counts  *counters
	flight  *flight
}

func New(store Cache, opts Options) *Handle {
	if store == nil {
		store = NewNoopStore()
	}
	if opts.Codec == nil {
		opts.Codec = JSONCodec{}
	}
	return &Handle{
		store:   store,
		alias:   opts.Alias,
		backend: opts.Backend,
		prefix:  opts.Prefix,
		version: opts.Version,
		ttl:     opts.TTL,
		codec:   opts.Codec,
		counts:  &counters{},
		flight:  newFlight(),
	}
}

func (h *Handle) Alias() string { return h.alias }

func (h *Handle) Store() Cache { return h.store }

func (h *Handle) Namespace() string {
	parts := make([]string, 0, 2)
	if h.prefix != "" {
		parts = append(parts, h.prefix)
	}
	if h.version != "" {
		parts = append(parts, h.version)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ":") + ":"
}

func (h *Handle) Key(key string) string { return h.Namespace() + key }

func (h *Handle) TTL(ttl time.Duration) time.Duration {
	switch {
	case ttl > 0:
		return ttl
	case ttl < 0:
		return 0
	case h.ttl > 0:
		return h.ttl
	default:
		return 0
	}
}

func (h *Handle) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, found, err := h.store.Get(ctx, h.Key(key))
	switch {
	case err != nil:
		h.counts.errors.Add(1)
		h.counts.misses.Add(1)
		return nil, false, err
	case found:
		h.counts.hits.Add(1)
	default:
		h.counts.misses.Add(1)
	}
	return value, found, nil
}

func (h *Handle) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := h.store.Set(ctx, h.Key(key), value, h.TTL(ttl)); err != nil {
		h.counts.errors.Add(1)
		return err
	}
	h.counts.sets.Add(1)
	return nil
}

func (h *Handle) Delete(ctx context.Context, key string) error {
	if err := h.store.Delete(ctx, h.Key(key)); err != nil {
		h.counts.errors.Add(1)
		return err
	}
	h.counts.deletes.Add(1)
	return nil
}

func (h *Handle) Has(ctx context.Context, key string) (bool, error) {
	found, err := h.store.Has(ctx, h.Key(key))
	if err != nil {
		h.counts.errors.Add(1)
	}
	return found, err
}

func (h *Handle) Clear(ctx context.Context) error {
	namespace := h.Namespace()
	if namespace == "" {
		return h.store.Clear(ctx)
	}
	return h.ClearPrefix(ctx, "")
}

func (h *Handle) ClearPrefix(ctx context.Context, prefix string) error {
	namespacer, ok := h.store.(Namespacer)
	if !ok {
		return h.store.Clear(ctx)
	}
	if err := namespacer.ClearPrefix(ctx, h.Key(prefix)); err != nil {
		h.counts.errors.Add(1)
		return err
	}
	return nil
}

func (h *Handle) Incr(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	counter, ok := h.store.(Counter)
	if !ok {
		return 0, ErrUnsupported
	}
	value, err := counter.Incr(ctx, h.Key(key), delta, h.TTL(ttl))
	if err != nil {
		h.counts.errors.Add(1)
	}
	return value, err
}

func (h *Handle) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	multi, ok := h.store.(Multi)
	if !ok {
		return nil, ErrUnsupported
	}
	namespaced := make([]string, 0, len(keys))
	for _, key := range keys {
		namespaced = append(namespaced, h.Key(key))
	}
	found, err := multi.GetMulti(ctx, namespaced)
	if err != nil {
		h.counts.errors.Add(1)
		return nil, err
	}

	namespace := h.Namespace()
	out := make(map[string][]byte, len(found))
	for key, value := range found {
		out[strings.TrimPrefix(key, namespace)] = value
	}
	for _, key := range keys {
		if _, hit := out[key]; hit {
			h.counts.hits.Add(1)
			continue
		}
		h.counts.misses.Add(1)
	}
	return out, nil
}

func (h *Handle) Ping(ctx context.Context) error { return Ping(ctx, h.store) }

func (h *Handle) Len() int { return Length(h.store) }

func (h *Handle) Close() error { return Close(h.store) }

func (h *Handle) Stats() Stats {
	stats := h.counts.snapshot()
	stats.Alias = h.alias
	stats.Backend = h.backend
	stats.Entries = Length(h.store)
	stats.Evictions = Evictions(h.store)
	return stats
}
