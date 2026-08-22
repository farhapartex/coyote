package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileOptions struct {
	Dir             string
	CleanupInterval time.Duration
}

type FileStore struct {
	dir      string
	mu       sync.Mutex
	stop     chan struct{}
	stopOnce sync.Once
}

func NewFileStore(opts FileOptions) *FileStore {
	store := &FileStore{dir: opts.Dir, stop: make(chan struct{})}
	if opts.CleanupInterval > 0 {
		go store.collect(opts.CleanupInterval)
	}
	return store
}

func (f *FileStore) Dir() string { return f.dir }

func (f *FileStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	path := entryPath(f.dir, key)

	entry, err := readEntry(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, false, nil
	}
	if entry.expired(time.Now().UnixMilli()) {
		_ = os.Remove(path)
		return nil, false, nil
	}
	return bytes.Clone(entry.value), true, nil
}

func (f *FileStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if err := writeEntry(entryPath(f.dir, key), encodeEntry(key, value, ttl)); err != nil {
		return fmt.Errorf("coyote/cache: writing %s: %w", key, err)
	}
	return nil
}

func (f *FileStore) Delete(_ context.Context, key string) error {
	err := os.Remove(entryPath(f.dir, key))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("coyote/cache: deleting %s: %w", key, err)
	}
	return nil
}

func (f *FileStore) Has(ctx context.Context, key string) (bool, error) {
	_, found, err := f.Get(ctx, key)
	return found, err
}

func (f *FileStore) Clear(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.RemoveAll(f.dir); err != nil {
		return fmt.Errorf("coyote/cache: clearing %s: %w", f.dir, err)
	}
	return nil
}

func (f *FileStore) ClearPrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		return f.Clear(ctx)
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	return walkEntries(f.dir, func(path string, entry storedEntry) error {
		if !strings.HasPrefix(entry.key, prefix) {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
}

func (f *FileStore) Incr(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	current := int64(0)
	raw, found, err := f.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if found {
		if parsed, err := strconv.ParseInt(string(raw), 10, 64); err == nil {
			current = parsed
		}
	}
	current += delta
	if err := f.Set(ctx, key, strconv.AppendInt(nil, current, 10), ttl); err != nil {
		return 0, err
	}
	return current, nil
}

func (f *FileStore) Ping(context.Context) error {
	if err := os.MkdirAll(f.dir, 0o750); err != nil {
		return fmt.Errorf("coyote/cache: %s cannot be created: %w", f.dir, err)
	}
	probe, err := os.CreateTemp(f.dir, ".probe-*")
	if err != nil {
		return fmt.Errorf("coyote/cache: %s is not writable: %w", f.dir, err)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

func (f *FileStore) Len() int {
	total := 0
	now := time.Now().UnixMilli()
	_ = walkEntries(f.dir, func(_ string, entry storedEntry) error {
		if !entry.expired(now) {
			total++
		}
		return nil
	})
	return total
}

func (f *FileStore) Sweep() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	removed := 0
	now := time.Now().UnixMilli()
	_ = walkEntries(f.dir, func(path string, entry storedEntry) error {
		if entry.expired(now) && os.Remove(path) == nil {
			removed++
		}
		return nil
	})
	return removed
}

func (f *FileStore) Close() error {
	f.stopOnce.Do(func() { close(f.stop) })
	return nil
}

func (f *FileStore) collect(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-f.stop:
			return
		case <-ticker.C:
			f.Sweep()
		}
	}
}
