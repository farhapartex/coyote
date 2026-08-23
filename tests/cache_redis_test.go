package tests

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/cache/redis"
	"github.com/farhapartex/coyote/core/settings"
)

func newRedisStore(t *testing.T, server *fakeRedis, mutate ...func(*redis.Options)) *redis.Store {
	t.Helper()
	opts := redis.Options{Address: server.Address(), PoolSize: 2}
	for _, fn := range mutate {
		fn(&opts)
	}
	store := redis.New(opts)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestRedisAgainstARealServer(t *testing.T) {
	address := os.Getenv("COYOTE_TEST_REDIS")
	if address == "" {
		t.Skip("set COYOTE_TEST_REDIS=host:port to run this against a real Redis")
	}

	ctx := context.Background()
	store := redis.New(redis.Options{Address: address, DialTimeout: 2 * time.Second})
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	prefix := "coyote:selftest:"
	t.Cleanup(func() { _ = store.ClearPrefix(ctx, prefix) })

	payload := []byte{0x00, 0x0d, 0x0a, 0xff, '$', '*'}
	if err := store.Set(ctx, prefix+"binary", payload, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, found, err := store.Get(ctx, prefix+"binary")
	if err != nil || !found || string(value) != string(payload) {
		t.Fatalf("Get = %v, %v, %v", value, found, err)
	}

	if err := store.Set(ctx, prefix+"brief", []byte("x"), 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	if _, found, _ := store.Get(ctx, prefix+"brief"); found {
		t.Error("PX should have expired the key")
	}

	if got, err := store.Incr(ctx, prefix+"counter", 2, time.Minute); err != nil || got != 2 {
		t.Errorf("Incr = %d, %v", got, err)
	}

	for i := range 12 {
		if err := store.Set(ctx, prefix+"bulk"+strconv.Itoa(i), []byte("x"), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ClearPrefix(ctx, prefix+"bulk"); err != nil {
		t.Fatalf("ClearPrefix: %v", err)
	}
	for i := range 12 {
		if _, found, _ := store.Get(ctx, prefix+"bulk"+strconv.Itoa(i)); found {
			t.Fatalf("bulk%d survived ClearPrefix", i)
		}
	}
	if _, found, _ := store.Get(ctx, prefix+"binary"); !found {
		t.Error("ClearPrefix removed a key outside its prefix")
	}
}

func TestRedisRoundTrip(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

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

	if found, err := store.Has(ctx, "greeting"); err != nil || !found {
		t.Errorf("Has = %v, %v", found, err)
	}
	if err := store.Delete(ctx, "greeting"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, found, _ := store.Get(ctx, "greeting"); found {
		t.Error("Get after Delete should miss")
	}
}

func TestRedisMissIsNotAnError(t *testing.T) {
	ctx := context.Background()
	store := newRedisStore(t, newFakeRedis(t))

	value, found, err := store.Get(ctx, "absent")
	if err != nil {
		t.Fatalf("a miss should not be an error, got %v", err)
	}
	if found || value != nil {
		t.Errorf("Get = %q, %v", value, found)
	}
}

func TestRedisStoresBinaryValuesIntact(t *testing.T) {
	ctx := context.Background()
	store := newRedisStore(t, newFakeRedis(t))

	payload := []byte{0x00, 0x0d, 0x0a, 0xff, '$', '*', 0x00}
	if err := store.Set(ctx, "binary", payload, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, found, err := store.Get(ctx, "binary")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(value) != string(payload) {
		t.Errorf("value = %v, want %v", value, payload)
	}
}

func TestRedisExpiryIsSentAsPX(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	if err := store.Set(ctx, "brief", []byte("x"), 20*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, found, _ := store.Get(ctx, "brief"); !found {
		t.Fatal("the key should be present before it expires")
	}
	time.Sleep(40 * time.Millisecond)
	if _, found, _ := store.Get(ctx, "brief"); found {
		t.Error("the key should have expired")
	}
}

func TestRedisClearPrefixScansAndDeletes(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	for i := range 5 {
		if err := store.Set(ctx, "one:v1:key"+strconv.Itoa(i), []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Set(ctx, "two:v1:keep", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearPrefix(ctx, "one:v1:"); err != nil {
		t.Fatalf("ClearPrefix: %v", err)
	}

	remaining := server.Keys()
	if len(remaining) != 1 || remaining[0] != "two:v1:keep" {
		t.Errorf("remaining keys = %v, want only the other namespace", remaining)
	}
}

func TestRedisClearPrefixEmptiesALargeNamespace(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	for i := range 25 {
		if err := store.Set(ctx, "one:v1:key"+strconv.Itoa(i), []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Set(ctx, "two:v1:keep", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearPrefix(ctx, "one:v1:"); err != nil {
		t.Fatalf("ClearPrefix: %v", err)
	}

	remaining := server.Keys()
	if len(remaining) != 1 || remaining[0] != "two:v1:keep" {
		t.Errorf("remaining keys = %v; clearing must survive deleting while scanning", remaining)
	}
}

func TestRedisClearPrefixNeverFlushes(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	if err := store.Set(ctx, "coyote:1:key", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.ClearPrefix(ctx, "coyote:1:"); err != nil {
		t.Fatal(err)
	}
	for _, command := range server.Commands() {
		if command == "FLUSHDB" || command == "FLUSHALL" {
			t.Fatalf("the backend issued %s; it must only delete its own keys", command)
		}
	}
}

func TestRedisClearPrefixEscapesGlobCharacters(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	if err := store.Set(ctx, "a*b:target", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "axb:other", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearPrefix(ctx, "a*b:"); err != nil {
		t.Fatalf("ClearPrefix: %v", err)
	}

	remaining := server.Keys()
	if len(remaining) != 1 || remaining[0] != "axb:other" {
		t.Errorf("remaining = %v; a literal * must not match any character", remaining)
	}
}

func TestRedisIncrAndExpire(t *testing.T) {
	ctx := context.Background()
	store := newRedisStore(t, newFakeRedis(t))

	value, err := store.Incr(ctx, "generation", 1, time.Minute)
	if err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if value != 1 {
		t.Errorf("first Incr = %d, want 1", value)
	}
	if value, err = store.Incr(ctx, "generation", 4, 0); err != nil || value != 5 {
		t.Errorf("second Incr = %d, %v", value, err)
	}
}

func TestRedisGetMulti(t *testing.T) {
	ctx := context.Background()
	store := newRedisStore(t, newFakeRedis(t))

	if err := store.Set(ctx, "a", []byte("1"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "c", []byte("3"), 0); err != nil {
		t.Fatal(err)
	}

	found, err := store.GetMulti(ctx, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("GetMulti: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("GetMulti = %v, want two entries", found)
	}
	if string(found["a"]) != "1" || string(found["c"]) != "3" {
		t.Errorf("GetMulti = %v", found)
	}
}

func TestRedisServerErrorIsReported(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)
	server.FailWith("ERR something went wrong")

	if _, _, err := store.Get(ctx, "key"); err == nil {
		t.Fatal("a server error reply should surface as an error")
	} else if !errors.Is(err, redis.ErrServer) {
		t.Errorf("error = %v, want ErrServer", err)
	}
}

func TestRedisRetriesOnceOnADroppedConnection(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	if err := store.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatal(err)
	}
	server.DropOn("GET")

	value, found, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("a dropped connection should be retried, got %v", err)
	}
	if !found || string(value) != "value" {
		t.Errorf("Get after a retry = %q, %v", value, found)
	}
}

func TestRedisReportsAnUnreachableServer(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	address := server.Address()
	server.Close()

	store := redis.New(redis.Options{Address: address, DialTimeout: 200 * time.Millisecond})
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Ping(ctx); err == nil {
		t.Fatal("Ping against a closed listener should fail")
	}
	if _, _, err := store.Get(ctx, "key"); err == nil {
		t.Error("Get against a closed listener should fail")
	}
}

func TestRedisHonoursTheReadTimeout(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server, func(o *redis.Options) {
		o.ReadTimeout = 30 * time.Millisecond
	})
	server.DelayBy(200 * time.Millisecond)

	started := time.Now()
	if _, _, err := store.Get(ctx, "key"); err == nil {
		t.Fatal("a slow reply should hit the read timeout")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("the timeout took %v to fire", elapsed)
	}
}

func TestRedisHonoursACancelledContext(t *testing.T) {
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := store.Get(ctx, "key"); err == nil {
		t.Error("a cancelled context should stop the command")
	}
}

func TestRedisAuthenticates(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	server.RequirePassword("hunter2")

	good := newRedisStore(t, server, func(o *redis.Options) { o.Password = "hunter2" })
	if err := good.Ping(ctx); err != nil {
		t.Fatalf("Ping with the right password: %v", err)
	}

	bad := newRedisStore(t, server, func(o *redis.Options) { o.Password = "wrong" })
	if err := bad.Ping(ctx); err == nil {
		t.Error("the wrong password should fail the handshake")
	}
}

func TestRedisSelectsTheDatabase(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server, func(o *redis.Options) { o.Database = 3 })

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	selected := false
	for _, command := range server.Commands() {
		if command == "SELECT" {
			selected = true
		}
	}
	if !selected {
		t.Error("a non-zero Database should issue SELECT on connect")
	}
}

func TestRedisPoolIsReusedAndBounded(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server, func(o *redis.Options) { o.PoolSize = 2 })

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key" + strconv.Itoa(n%4)
			if err := store.Set(ctx, key, []byte("v"), 0); err != nil {
				t.Errorf("Set: %v", err)
			}
			if _, _, err := store.Get(ctx, key); err != nil {
				t.Errorf("Get: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestRedisUseAfterCloseFails(t *testing.T) {
	ctx := context.Background()
	store := redis.New(redis.Options{Address: newFakeRedis(t).Address()})

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, _, err := store.Get(ctx, "key"); !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Get after Close = %v, want ErrClosed", err)
	}
}

func TestRedisReportsNoEntryCount(t *testing.T) {
	store := newRedisStore(t, newFakeRedis(t))
	if store.Len() != cache.UnknownLength {
		t.Errorf("Len = %d, want %d for Redis", store.Len(), cache.UnknownLength)
	}
}

func TestRedisThroughAHandleIsNamespaced(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)
	store := newRedisStore(t, server)

	handle := cache.New(store, cache.Options{Alias: "default", Prefix: "coyote", Version: "7"})
	if err := handle.Set(ctx, "totals", []byte("1"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	keys := server.Keys()
	if len(keys) != 1 || keys[0] != "coyote:7:totals" {
		t.Errorf("stored keys = %v, want the namespaced key", keys)
	}
}

func TestRedisThroughAnAppIsUsable(t *testing.T) {
	ctx := context.Background()
	server := newFakeRedis(t)

	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{
			Backend:     settings.CacheInRedis,
			Address:     server.Address(),
			DialTimeout: time.Second,
		}}
	})

	c := a.Cache()
	if err := c.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, found, err := c.Get(ctx, "key")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(value) != "value" {
		t.Errorf("value = %q", value)
	}

	statuses := a.ProbeCaches(ctx)
	if len(statuses) != 1 || !statuses[0].Reachable {
		t.Errorf("ProbeCaches = %+v, want a reachable Redis", statuses)
	}
	if statuses[0].Address != server.Address() {
		t.Errorf("Address = %q, want %q", statuses[0].Address, server.Address())
	}
}

func TestRedisProbeReportsAnUnreachableServer(t *testing.T) {
	server := newFakeRedis(t)
	address := server.Address()
	server.Close()

	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{
			Backend:     settings.CacheInRedis,
			Address:     address,
			DialTimeout: 200 * time.Millisecond,
		}}
	})

	statuses := a.ProbeCaches(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("ProbeCaches = %d entries, want 1", len(statuses))
	}
	if statuses[0].Reachable || statuses[0].Err == nil {
		t.Errorf("an unreachable Redis should report an error: %+v", statuses[0])
	}
}

func TestRedisPasswordIsNotInTheProbeAddress(t *testing.T) {
	server := newFakeRedis(t)
	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{
			Backend:  settings.CacheInRedis,
			Address:  server.Address(),
			Password: "hunter2",
		}}
	})

	for _, status := range a.ProbeCaches(context.Background()) {
		if status.Address == "hunter2" {
			t.Error("the probe must not carry the password")
		}
	}
}
