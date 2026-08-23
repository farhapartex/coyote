package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/settings"
)

const cacheProbeTimeout = 10 * time.Second

type UnreachableCache struct {
	Status cache.Status
	Index  int
}

func (e *UnreachableCache) Unwrap() error { return e.Status.Err }

func (e *UnreachableCache) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "coyote/cache: the %s cache is unreachable\n", strconv.Quote(e.Status.Alias))
	fmt.Fprintf(&b, "    backend %s", e.Status.Backend)
	if e.Status.Address != "" {
		fmt.Fprintf(&b, " at %s", e.Status.Address)
	}
	if e.Status.Err != nil {
		fmt.Fprintf(&b, "\n    %s", e.Status.Err)
	}
	fmt.Fprintf(&b, "\n\n  %s", e.hint())
	return b.String()
}

func (e *UnreachableCache) hint() string {
	field := "Caches[" + strconv.Itoa(e.Index) + "].Backend"
	switch settings.CacheBackend(e.Status.Backend) {
	case settings.CacheInRedis:
		return "start Redis, or set " + field + " to \"memory\" or \"file\" in settings.go"
	case settings.CacheInFile:
		return "make that directory writable, or set " + field + " to \"memory\" in settings.go"
	default:
		return "check " + field + " in settings.go"
	}
}

func (a *App) CheckCaches(ctx context.Context) error {
	probe, cancel := context.WithTimeout(ctx, cacheProbeTimeout)
	defer cancel()

	statuses := a.ProbeCaches(probe)
	for i, status := range statuses {
		if !status.Reachable {
			return &UnreachableCache{Status: status, Index: i}
		}
	}
	for _, status := range statuses {
		a.Logger.Debug("cache ready", "alias", status.Alias, "backend", status.Backend)
	}
	return nil
}
