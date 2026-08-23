package cache

import (
	"bytes"
	"container/list"
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type MemoryOptions struct {
	MaxEntries      int
	MaxBytes        int64
	CleanupInterval time.Duration
}

type memoryEntry struct {
	key     string
	value   []byte
	expires time.Time
}

type MemoryStore struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	order      *list.List
	bytes      int64
	maxEntries int
	maxBytes   int64
	evictions  atomic.Int64
	stop       chan struct{}
	stopOnce   sync.Once
}

func NewMemoryStore(opts MemoryOptions) *MemoryStore {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = DefaultMaxEntries
	}
	store := &MemoryStore{
		entries:    make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: opts.MaxEntries,
		maxBytes:   opts.MaxBytes,
		stop:       make(chan struct{}),
	}
	if opts.CleanupInterval > 0 {
		go store.collect(opts.CleanupInterval)
	}
	return store
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	element, found := m.entries[key]
	if !found {
		return nil, false, nil
	}
	entry := element.Value.(*memoryEntry)
	if expired(entry, time.Now()) {
		m.remove(element)
		return nil, false, nil
	}
	m.order.MoveToFront(element)
	return bytes.Clone(entry.value), true, nil
}

func (m *MemoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store(key, bytes.Clone(value), ttl)
	return nil
}

func (m *MemoryStore) store(key string, value []byte, ttl time.Duration) {
	var expires time.Time
	if ttl > 0 {
		expires = time.Now().Add(ttl)
	}
	if element, found := m.entries[key]; found {
		m.remove(element)
	}
	entry := &memoryEntry{key: key, value: value, expires: expires}
	m.entries[key] = m.order.PushFront(entry)
	m.bytes += int64(len(value))
	m.evict()
}

func (m *MemoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if element, found := m.entries[key]; found {
		m.remove(element)
	}
	return nil
}

func (m *MemoryStore) Has(ctx context.Context, key string) (bool, error) {
	_, found, err := m.Get(ctx, key)
	return found, err
}

func (m *MemoryStore) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*list.Element)
	m.order.Init()
	m.bytes = 0
	return nil
}

func (m *MemoryStore) ClearPrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, element := range m.entries {
		if strings.HasPrefix(key, prefix) {
			m.remove(element)
		}
	}
	return nil
}

func (m *MemoryStore) Incr(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := int64(0)
	if element, found := m.entries[key]; found {
		entry := element.Value.(*memoryEntry)
		if expired(entry, time.Now()) {
			m.remove(element)
		} else if parsed, err := strconv.ParseInt(string(entry.value), 10, 64); err == nil {
			current = parsed
		}
	}
	current += delta
	m.store(key, strconv.AppendInt(nil, current, 10), ttl)
	return current, nil
}

func (m *MemoryStore) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(keys))
	for _, key := range keys {
		if value, found, _ := m.Get(ctx, key); found {
			out[key] = value
		}
	}
	return out, nil
}

func (m *MemoryStore) Ping(context.Context) error { return nil }

func (m *MemoryStore) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func (m *MemoryStore) Bytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytes
}

func (m *MemoryStore) Evictions() int64 { return m.evictions.Load() }

func (m *MemoryStore) Close() error {
	m.stopOnce.Do(func() { close(m.stop) })
	return nil
}

func (m *MemoryStore) remove(element *list.Element) {
	entry := element.Value.(*memoryEntry)
	m.order.Remove(element)
	delete(m.entries, entry.key)
	m.bytes -= int64(len(entry.value))
}

func (m *MemoryStore) evict() {
	for m.order.Len() > 0 && m.overCapacity() {
		oldest := m.order.Back()
		if oldest == nil {
			return
		}
		m.remove(oldest)
		m.evictions.Add(1)
	}
}

func (m *MemoryStore) overCapacity() bool {
	if len(m.entries) > m.maxEntries {
		return true
	}
	return m.maxBytes > 0 && m.bytes > m.maxBytes
}

func (m *MemoryStore) collect(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.sweep(time.Now())
		}
	}
}

func (m *MemoryStore) sweep(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, element := range m.entries {
		if expired(element.Value.(*memoryEntry), now) {
			m.remove(element)
		}
	}
}

func expired(entry *memoryEntry, now time.Time) bool {
	return !entry.expires.IsZero() && now.After(entry.expires)
}
