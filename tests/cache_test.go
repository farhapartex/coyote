package tests

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/cache"
)

func newMemoryHandle(opts cache.Options) *cache.Handle {
	return cache.New(cache.NewMemoryStore(cache.MemoryOptions{}), opts)
}

func TestCacheSetGetDelete(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{Alias: "default", Prefix: "coyote"})

	if _, found, err := c.Get(ctx, "absent"); err != nil || found {
		t.Fatalf("Get on an empty cache = %v, %v", found, err)
	}
	if err := c.Set(ctx, "greeting", []byte("hello"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	value, found, err := c.Get(ctx, "greeting")
	if err != nil || !found {
		t.Fatalf("Get after Set = %v, %v", found, err)
	}
	if string(value) != "hello" {
		t.Errorf("value = %q, want %q", value, "hello")
	}

	if found, _ := c.Has(ctx, "greeting"); !found {
		t.Error("Has should report a stored key")
	}
	if err := c.Delete(ctx, "greeting"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if found, _ := c.Has(ctx, "greeting"); found {
		t.Error("Has should not report a deleted key")
	}
}

func TestCacheKeysAreNamespaced(t *testing.T) {
	ctx := context.Background()
	store := cache.NewMemoryStore(cache.MemoryOptions{})
	c := cache.New(store, cache.Options{Prefix: "coyote", Version: "v3"})

	if err := c.Set(ctx, "totals", []byte("1"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, "coyote:v3:totals"); !found {
		t.Error("the backend should hold the namespaced key")
	}
	if _, found, _ := store.Get(ctx, "totals"); found {
		t.Error("the backend should not hold the bare key")
	}
}

func TestCacheAliasesShareABackendWithoutColliding(t *testing.T) {
	ctx := context.Background()
	store := cache.NewMemoryStore(cache.MemoryOptions{})

	first := cache.New(store, cache.Options{Alias: "one", Prefix: "one"})
	second := cache.New(store, cache.Options{Alias: "two", Prefix: "two"})

	if err := first.Set(ctx, "shared", []byte("a"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := second.Set(ctx, "shared", []byte("b"), time.Minute); err != nil {
		t.Fatal(err)
	}

	value, _, _ := first.Get(ctx, "shared")
	if string(value) != "a" {
		t.Errorf("first alias read %q, want %q", value, "a")
	}

	if err := first.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, found, _ := first.Get(ctx, "shared"); found {
		t.Error("Clear should empty its own namespace")
	}
	if _, found, _ := second.Get(ctx, "shared"); !found {
		t.Error("Clear must not touch another alias sharing the backend")
	}
}

func TestCacheVersionBumpHidesOldKeys(t *testing.T) {
	ctx := context.Background()
	store := cache.NewMemoryStore(cache.MemoryOptions{})

	before := cache.New(store, cache.Options{Prefix: "coyote", Version: "1"})
	if err := before.Set(ctx, "totals", []byte("old"), time.Minute); err != nil {
		t.Fatal(err)
	}

	after := cache.New(store, cache.Options{Prefix: "coyote", Version: "2"})
	if _, found, _ := after.Get(ctx, "totals"); found {
		t.Error("a version bump should make the old key unreachable")
	}
	if _, found, _ := before.Get(ctx, "totals"); !found {
		t.Error("the old version should still read its own key")
	}
}

func TestCacheDefaultTTLAndForever(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{TTL: 30 * time.Millisecond})

	if err := c.Set(ctx, "lapses", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ctx, "stays", []byte("x"), cache.Forever); err != nil {
		t.Fatal(err)
	}

	time.Sleep(60 * time.Millisecond)

	if _, found, _ := c.Get(ctx, "lapses"); found {
		t.Error("a zero ttl should take the cache default and expire")
	}
	if _, found, _ := c.Get(ctx, "stays"); !found {
		t.Error("cache.Forever should not expire")
	}
}

func TestCacheExpiredKeyMisses(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	if err := c.Set(ctx, "brief", []byte("x"), 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, found, _ := c.Get(ctx, "brief"); found {
		t.Error("an expired key should miss")
	}
}

func TestCacheValuesAreCopiedInAndOut(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	original := []byte("hello")
	if err := c.Set(ctx, "key", original, time.Minute); err != nil {
		t.Fatal(err)
	}
	original[0] = 'j'

	stored, _, _ := c.Get(ctx, "key")
	if string(stored) != "hello" {
		t.Errorf("mutating the caller's slice changed the cache: %q", stored)
	}

	stored[0] = 'k'
	again, _, _ := c.Get(ctx, "key")
	if string(again) != "hello" {
		t.Errorf("mutating a returned slice changed the cache: %q", again)
	}
}

func TestMemoryStoreEvictsLeastRecentlyUsed(t *testing.T) {
	ctx := context.Background()
	store := cache.NewMemoryStore(cache.MemoryOptions{MaxEntries: 2})

	for _, key := range []string{"a", "b"} {
		if err := store.Set(ctx, key, []byte(key), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, _ := store.Get(ctx, "a"); !found {
		t.Fatal("a should still be present")
	}
	if err := store.Set(ctx, "c", []byte("c"), time.Minute); err != nil {
		t.Fatal(err)
	}

	if _, found, _ := store.Get(ctx, "b"); found {
		t.Error("b was least recently used and should have been evicted")
	}
	if _, found, _ := store.Get(ctx, "a"); !found {
		t.Error("a was touched most recently and should have survived")
	}
	if store.Evictions() != 1 {
		t.Errorf("Evictions = %d, want 1", store.Evictions())
	}
	if store.Len() != 2 {
		t.Errorf("Len = %d, want 2", store.Len())
	}
}

func TestMemoryStoreRespectsMaxBytes(t *testing.T) {
	ctx := context.Background()
	store := cache.NewMemoryStore(cache.MemoryOptions{MaxBytes: 10})

	for i := range 5 {
		if err := store.Set(ctx, strconv.Itoa(i), []byte("aaaa"), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if store.Bytes() > 10 {
		t.Errorf("Bytes = %d, want no more than 10", store.Bytes())
	}
	if store.Evictions() == 0 {
		t.Error("exceeding MaxBytes should evict")
	}
}

func TestCacheRememberBuildsOnceThenHits(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	type totals struct {
		Products int
		Revenue  float64
	}

	builds := 0
	build := func(context.Context) (totals, error) {
		builds++
		return totals{Products: 12, Revenue: 19.5}, nil
	}

	first, err := cache.Remember(ctx, c, "totals", time.Minute, build)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	second, err := cache.Remember(ctx, c, "totals", time.Minute, build)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	if builds != 1 {
		t.Errorf("build ran %d times, want 1", builds)
	}
	if first != second {
		t.Errorf("second read = %+v, want %+v", second, first)
	}
	if second.Products != 12 || second.Revenue != 19.5 {
		t.Errorf("decoded value = %+v", second)
	}
}

func TestCacheRememberKeepsIntegersIntact(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	build := func(context.Context) (int, error) { return 42, nil }
	if _, err := cache.Remember(ctx, c, "count", time.Minute, build); err != nil {
		t.Fatal(err)
	}
	value, err := cache.Remember(ctx, c, "count", time.Minute, func(context.Context) (int, error) {
		return 0, errors.New("should not rebuild")
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if value != 42 {
		t.Errorf("value = %d, want 42", value)
	}
}

func TestCacheRememberDoesNotCacheAFailedBuild(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	failure := errors.New("database is down")
	if _, err := cache.Remember(ctx, c, "totals", time.Minute, func(context.Context) (int, error) {
		return 0, failure
	}); !errors.Is(err, failure) {
		t.Fatalf("Remember error = %v, want %v", err, failure)
	}

	if found, _ := c.Has(ctx, "totals"); found {
		t.Error("a failed build must not populate the cache")
	}

	value, err := cache.Remember(ctx, c, "totals", time.Minute, func(context.Context) (int, error) {
		return 7, nil
	})
	if err != nil || value != 7 {
		t.Errorf("a later build should succeed, got %d, %v", value, err)
	}
}

func TestCacheRememberCollapsesConcurrentBuilds(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	var mu sync.Mutex
	builds := 0
	build := func(context.Context) (int, error) {
		mu.Lock()
		builds++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return 5, nil
	}

	var wg sync.WaitGroup
	values := make([]int, 10)
	for i := range values {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			value, err := cache.Remember(ctx, c, "totals", time.Minute, build)
			if err != nil {
				t.Errorf("Remember: %v", err)
			}
			values[slot] = value
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if builds != 1 {
		t.Errorf("build ran %d times under 10 concurrent misses, want 1", builds)
	}
	for i, value := range values {
		if value != 5 {
			t.Errorf("caller %d saw %d, want 5", i, value)
		}
	}
}

func TestCacheGetOrSetReturnsTheValueWhenTheCacheIsDead(t *testing.T) {
	ctx := context.Background()
	c := cache.New(cache.NewNoopStore(), cache.Options{})

	value, err := cache.GetOrSet(ctx, c, "key", time.Minute, func(context.Context) ([]byte, error) {
		return []byte("built"), nil
	})
	if err != nil {
		t.Fatalf("GetOrSet: %v", err)
	}
	if string(value) != "built" {
		t.Errorf("value = %q, want %q", value, "built")
	}
}

func TestCacheJSONHelpers(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	type product struct {
		Name  string
		Stock int
	}

	if err := cache.SetJSON(ctx, c, "p1", product{Name: "Kettle", Stock: 3}, time.Minute); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}
	value, found, err := cache.GetJSON[product](ctx, c, "p1")
	if err != nil || !found {
		t.Fatalf("GetJSON = %v, %v", found, err)
	}
	if value.Name != "Kettle" || value.Stock != 3 {
		t.Errorf("value = %+v", value)
	}

	if _, found, err := cache.GetJSON[product](ctx, c, "absent"); found || err != nil {
		t.Errorf("GetJSON on a miss = %v, %v", found, err)
	}
}

func TestCacheIncrementsAndCounts(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	value, err := c.Incr(ctx, "generation", 1, time.Minute)
	if err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if value != 1 {
		t.Errorf("first Incr = %d, want 1", value)
	}
	if value, _ = c.Incr(ctx, "generation", 2, time.Minute); value != 3 {
		t.Errorf("second Incr = %d, want 3", value)
	}
}

func TestCacheIncrIsUnsupportedWithoutACounter(t *testing.T) {
	c := cache.New(cache.NewNoopStore(), cache.Options{})
	if _, err := c.Incr(context.Background(), "key", 1, 0); !errors.Is(err, cache.ErrUnsupported) {
		t.Errorf("Incr error = %v, want ErrUnsupported", err)
	}
}

func TestCacheStatsCountHitsAndMisses(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{Alias: "default", Backend: "memory"})

	if _, _, err := c.Get(ctx, "absent"); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ctx, "key", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Get(ctx, "key"); err != nil {
		t.Fatal(err)
	}

	stats := c.Stats()
	if stats.Alias != "default" || stats.Backend != "memory" {
		t.Errorf("stats identity = %q/%q", stats.Alias, stats.Backend)
	}
	if stats.Hits != 1 || stats.Misses != 1 || stats.Sets != 1 {
		t.Errorf("stats = %+v", stats)
	}
	if stats.HitRate() != 0.5 {
		t.Errorf("HitRate = %v, want 0.5", stats.HitRate())
	}
	if stats.Entries != 1 {
		t.Errorf("Entries = %d, want 1", stats.Entries)
	}
}

func TestCacheStatsHitRateIsZeroWithoutLookups(t *testing.T) {
	if rate := (cache.Stats{}).HitRate(); rate != 0 {
		t.Errorf("HitRate = %v, want 0", rate)
	}
}

func TestCacheGetMultiStripsTheNamespace(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{Prefix: "coyote", Version: "1"})

	if err := c.Set(ctx, "a", []byte("1"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ctx, "b", []byte("2"), time.Minute); err != nil {
		t.Fatal(err)
	}

	found, err := c.GetMulti(ctx, []string{"a", "b", "missing"})
	if err != nil {
		t.Fatalf("GetMulti: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("GetMulti returned %d entries, want 2", len(found))
	}
	if string(found["a"]) != "1" || string(found["b"]) != "2" {
		t.Errorf("GetMulti = %v", found)
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	c := newMemoryHandle(cache.Options{})

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key" + strconv.Itoa(n%5)
			if err := c.Set(ctx, key, []byte(strconv.Itoa(n)), time.Minute); err != nil {
				t.Errorf("Set: %v", err)
			}
			c.Get(ctx, key)
			if n%7 == 0 {
				c.Clear(ctx)
			}
		}(i)
	}
	wg.Wait()
}
