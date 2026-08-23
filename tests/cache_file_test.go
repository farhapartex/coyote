package tests

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/settings"
)

func newFileStore(t *testing.T) *cache.FileStore {
	t.Helper()
	store := cache.NewFileStore(cache.FileOptions{Dir: filepath.Join(t.TempDir(), "cache")})
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestFileCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	if _, found, err := store.Get(ctx, "absent"); err != nil || found {
		t.Fatalf("Get on an empty cache = %v, %v", found, err)
	}
	if err := store.Set(ctx, "greeting", []byte("hello"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	value, found, err := store.Get(ctx, "greeting")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(value) != "hello" {
		t.Errorf("value = %q, want %q", value, "hello")
	}

	if err := store.Delete(ctx, "greeting"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if found, _ := store.Has(ctx, "greeting"); found {
		t.Error("Has after Delete should be false")
	}
}

func TestFileCacheStoresBinaryValues(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	payload := []byte{0x00, 0xff, 0x0a, 0x0d, 0x00}
	if err := store.Set(ctx, "binary", payload, 0); err != nil {
		t.Fatal(err)
	}
	value, found, err := store.Get(ctx, "binary")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(value) != string(payload) {
		t.Errorf("value = %v, want %v", value, payload)
	}
}

func TestFileCacheExpires(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	if err := store.Set(ctx, "brief", []byte("x"), 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, "brief"); !found {
		t.Fatal("the key should exist before expiry")
	}
	time.Sleep(40 * time.Millisecond)
	if _, found, _ := store.Get(ctx, "brief"); found {
		t.Error("an expired entry should miss")
	}
}

func TestFileCacheKeepsForeverEntries(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	if err := store.Set(ctx, "permanent", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if removed := store.Sweep(); removed != 0 {
		t.Errorf("Sweep removed %d entries without a ttl", removed)
	}
	if _, found, _ := store.Get(ctx, "permanent"); !found {
		t.Error("an entry with no ttl should survive a sweep")
	}
}

func TestFileCacheSweepRemovesExpiredEntries(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	if err := store.Set(ctx, "brief", []byte("x"), 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "lasting", []byte("x"), time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)

	if removed := store.Sweep(); removed != 1 {
		t.Errorf("Sweep removed %d entries, want 1", removed)
	}
	if store.Len() != 1 {
		t.Errorf("Len = %d, want 1", store.Len())
	}
}

func TestFileCacheClearPrefixKeepsOtherNamespaces(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	for i := range 4 {
		if err := store.Set(ctx, "one:v1:key"+strconv.Itoa(i), []byte("x"), time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Set(ctx, "two:v1:keep", []byte("x"), time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearPrefix(ctx, "one:v1:"); err != nil {
		t.Fatalf("ClearPrefix: %v", err)
	}
	if store.Len() != 1 {
		t.Errorf("Len = %d, want only the other namespace", store.Len())
	}
	if _, found, _ := store.Get(ctx, "two:v1:keep"); !found {
		t.Error("the other namespace should survive")
	}
}

func TestFileCacheClearEmptiesEverything(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	if err := store.Set(ctx, "key", []byte("x"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if store.Len() != 0 {
		t.Errorf("Len after Clear = %d, want 0", store.Len())
	}
	if err := store.Set(ctx, "key", []byte("y"), time.Hour); err != nil {
		t.Errorf("the cache should be usable after Clear: %v", err)
	}
}

func TestFileCacheKeysNeverEscapeTheDirectory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dir := filepath.Join(root, "cache")
	store := cache.NewFileStore(cache.FileOptions{Dir: dir})
	t.Cleanup(func() { _ = store.Close() })

	hostile := []string{
		"../../etc/passwd",
		"..\\..\\windows",
		strings.Repeat("a/", 40) + "deep",
		"key\x00null",
	}
	for _, key := range hostile {
		if err := store.Set(ctx, key, []byte("x"), time.Minute); err != nil {
			t.Fatalf("Set(%q): %v", key, err)
		}
		value, found, err := store.Get(ctx, key)
		if err != nil || !found || string(value) != "x" {
			t.Errorf("Get(%q) = %q, %v, %v", key, value, found, err)
		}
	}

	var strays []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
			strays = append(strays, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(strays) > 0 {
		t.Errorf("files written outside the cache directory: %v", strays)
	}
}

func TestFileCacheIgnoresACorruptEntry(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "cache")
	store := cache.NewFileStore(cache.FileOptions{Dir: dir})
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Set(ctx, "key", []byte("value"), time.Hour); err != nil {
		t.Fatal(err)
	}

	truncated := 0
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".cache") {
			return err
		}
		truncated++
		return os.WriteFile(path, []byte{0x00, 0x01}, 0o640)
	})
	if err != nil {
		t.Fatal(err)
	}
	if truncated == 0 {
		t.Fatal("no cache file was found to truncate")
	}

	if _, found, err := store.Get(ctx, "key"); err != nil || found {
		t.Errorf("a truncated entry should miss cleanly, got %v, %v", found, err)
	}
	if err := store.Set(ctx, "key", []byte("again"), time.Hour); err != nil {
		t.Fatalf("the cache should recover: %v", err)
	}
	value, found, _ := store.Get(ctx, "key")
	if !found || string(value) != "again" {
		t.Errorf("value = %q, %v", value, found)
	}
}

func TestFileCacheIncrements(t *testing.T) {
	ctx := context.Background()
	store := newFileStore(t)

	if value, err := store.Incr(ctx, "generation", 1, time.Hour); err != nil || value != 1 {
		t.Fatalf("first Incr = %d, %v", value, err)
	}
	if value, err := store.Incr(ctx, "generation", 2, time.Hour); err != nil || value != 3 {
		t.Errorf("second Incr = %d, %v", value, err)
	}
}

func TestFileCachePingReportsAnUnwritableDirectory(t *testing.T) {
	ctx := context.Background()

	writable := newFileStore(t)
	if err := writable.Ping(ctx); err != nil {
		t.Fatalf("Ping on a writable directory: %v", err)
	}

	parent := t.TempDir()
	blocked := filepath.Join(parent, "denied")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	store := cache.NewFileStore(cache.FileOptions{Dir: filepath.Join(blocked, "cache")})
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Ping(ctx); err == nil {
		t.Error("Ping should report a directory it cannot write to")
	}
}

func TestFileCacheThroughAnAppIsNamespaced(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "pages")

	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{
			{Backend: settings.CacheInMemory},
			{Alias: "pages", Backend: settings.CacheInFile, Dir: dir, Prefix: "pages"},
		}
	})

	pages, err := a.CacheByAlias("pages")
	if err != nil {
		t.Fatalf("CacheByAlias: %v", err)
	}
	if err := pages.Set(ctx, "home", []byte("<html>"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	value, found, err := pages.Get(ctx, "home")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(value) != "<html>" {
		t.Errorf("value = %q", value)
	}
	if _, found, _ := a.Cache().Get(ctx, "home"); found {
		t.Error("the default cache should not see the file cache's key")
	}

	statuses := a.ProbeCaches(ctx)
	for _, status := range statuses {
		if !status.Reachable {
			t.Errorf("cache %q should be reachable: %v", status.Alias, status.Err)
		}
	}
}
