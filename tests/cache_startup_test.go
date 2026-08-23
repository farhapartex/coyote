package tests

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/settings"
)

func TestStartupAcceptsAMemoryCache(t *testing.T) {
	a := newTestApp(t)
	if err := a.CheckCaches(context.Background()); err != nil {
		t.Errorf("an in-memory cache should never block startup: %v", err)
	}
}

func TestStartupRefusesAnUnreachableRedis(t *testing.T) {
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

	err := a.CheckCaches(context.Background())
	if err == nil {
		t.Fatal("a configured Redis that is not running must stop startup")
	}

	var unreachable *app.UnreachableCache
	if !errors.As(err, &unreachable) {
		t.Fatalf("error = %T, want *app.UnreachableCache", err)
	}
	if unreachable.Status.Alias != "default" || unreachable.Status.Backend != "redis" {
		t.Errorf("status = %+v", unreachable.Status)
	}

	message := err.Error()
	for _, want := range []string{"unreachable", "redis", address, "settings.go", "\"memory\""} {
		if !strings.Contains(message, want) {
			t.Errorf("message should mention %q:\n%s", want, message)
		}
	}
}

func TestStartupErrorNamesTheOffendingCacheIndex(t *testing.T) {
	server := newFakeRedis(t)
	address := server.Address()
	server.Close()

	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{
			{Backend: settings.CacheInMemory},
			{Alias: "sessions", Backend: settings.CacheInRedis, Address: address, DialTimeout: 200 * time.Millisecond},
		}
	})

	err := a.CheckCaches(context.Background())
	if err == nil {
		t.Fatal("the second cache is unreachable, so startup must fail")
	}
	if !strings.Contains(err.Error(), "Caches[1].Backend") {
		t.Errorf("the message should point at the offending entry:\n%s", err)
	}
	if !strings.Contains(err.Error(), "\"sessions\"") {
		t.Errorf("the message should name the alias:\n%s", err)
	}
}

func TestStartupRefusesAnUnwritableFileCache(t *testing.T) {
	parent := t.TempDir()
	blocked := filepath.Join(parent, "denied")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{
			Backend: settings.CacheInFile,
			Dir:     filepath.Join(blocked, "cache"),
		}}
	})

	err := a.CheckCaches(context.Background())
	if err == nil {
		t.Fatal("a file cache in an unwritable directory must stop startup")
	}
	if !strings.Contains(err.Error(), "writable") {
		t.Errorf("the message should say the directory is the problem:\n%s", err)
	}
}

func TestStartupAcceptsAReachableRedis(t *testing.T) {
	server := newFakeRedis(t)
	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{
			Backend:     settings.CacheInRedis,
			Address:     server.Address(),
			DialTimeout: time.Second,
		}}
	})

	if err := a.CheckCaches(context.Background()); err != nil {
		t.Errorf("a running Redis should pass the check: %v", err)
	}
}

func TestStartupRedactsThePasswordInItsError(t *testing.T) {
	server := newFakeRedis(t)
	address := server.Address()
	server.Close()

	a := newTestApp(t, func(s *settings.Settings) {
		s.Caches = []settings.Cache{{
			Backend:     settings.CacheInRedis,
			Address:     address,
			Password:    "hunter2",
			DialTimeout: 200 * time.Millisecond,
		}}
	})

	err := a.CheckCaches(context.Background())
	if err == nil {
		t.Fatal("expected the check to fail")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("the password must not appear in the error:\n%s", err)
	}
}

func TestServeRefusesToListenWithAnUnreachableCache(t *testing.T) {
	server := newFakeRedis(t)
	redisAddress := server.Address()
	server.Close()

	serverAddress, release := listenOnFreePort(t)
	release()
	host, portText, err := net.SplitHostPort(serverAddress)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	a := newTestApp(t, func(s *settings.Settings) {
		s.Server.Host = host
		s.Server.Port = port
		s.Caches = []settings.Cache{{
			Backend:     settings.CacheInRedis,
			Address:     redisAddress,
			DialTimeout: 200 * time.Millisecond,
		}}
	})

	done := make(chan error, 1)
	go func() { done <- a.Serve() }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Serve should refuse to start")
		}
		var unreachable *app.UnreachableCache
		if !errors.As(err, &unreachable) {
			t.Errorf("Serve error = %v, want *app.UnreachableCache", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return; it should have refused before listening")
	}

	if cli.ServerRunning(serverAddress) {
		t.Error("no listener should have been opened")
	}
}
