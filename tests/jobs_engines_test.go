package tests

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/store"
	"gorm.io/gorm"
)

const (
	EnvPostgresDSN = "COYOTE_TEST_POSTGRES_DSN"
	EnvMySQLDSN    = "COYOTE_TEST_MYSQL_DSN"
)

type engine struct {
	name  string
	apply func(*settings.Settings)
	skip  string
}

func engines(t *testing.T) []engine {
	t.Helper()
	out := []engine{{name: "sqlite"}}

	for _, candidate := range []struct {
		name   string
		env    string
		engine settings.Engine
	}{
		{"postgres", EnvPostgresDSN, settings.Postgres},
		{"mysql", EnvMySQLDSN, settings.MySQL},
	} {
		raw := strings.TrimSpace(os.Getenv(candidate.env))
		if raw == "" {
			out = append(out, engine{
				name: candidate.name,
				skip: "set " + candidate.env + " to a connection URL to run the " + candidate.name + " checks",
			})
			continue
		}
		database, err := databaseFromURL(candidate.engine, raw)
		if err != nil {
			out = append(out, engine{name: candidate.name, skip: candidate.env + " is not usable: " + err.Error()})
			continue
		}
		out = append(out, engine{name: candidate.name, apply: func(s *settings.Settings) {
			s.Databases = []settings.Database{database}
		}})
	}
	return out
}

func databaseFromURL(kind settings.Engine, raw string) (settings.Database, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return settings.Database{}, err
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if name == "" {
		return settings.Database{}, errors.New("the URL names no database")
	}

	port := 5432
	if kind == settings.MySQL {
		port = 3306
	}
	if given := parsed.Port(); given != "" {
		if number, err := strconv.Atoi(given); err == nil {
			port = number
		}
	}
	host := parsed.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	password, _ := parsed.User.Password()

	return settings.Database{
		Alias:        "default",
		Engine:       kind,
		Name:         name,
		Host:         host,
		Port:         port,
		User:         parsed.User.Username(),
		Password:     password,
		MaxOpenConns: 4,
	}, nil
}

func onEachEngine(t *testing.T, scenario func(*testing.T, jobs.Queue, *gorm.DB)) {
	t.Helper()
	for _, target := range engines(t) {
		t.Run(target.name, func(t *testing.T) {
			if target.skip != "" {
				t.Skip(target.skip)
			}

			fns := []func(*settings.Settings){func(s *settings.Settings) { s.Jobs.Enabled = true }}
			if target.apply != nil {
				fns = append(fns, target.apply)
			}
			a := app.NewFrom(devSettings(t, fns...))
			a.RegisterModel(model.Of(jobs.Record{}))
			syncSchema(t, a)
			t.Cleanup(func() { _ = a.CloseDB() })

			handle, err := a.DB()
			if err != nil {
				t.Fatalf("opening %s: %v", target.name, err)
			}
			if err := handle.Where("1 = 1").Delete(&jobs.Record{}).Error; err != nil {
				t.Fatalf("clearing the jobs table on %s: %v", target.name, err)
			}

			scenario(t, store.Jobs(handle, store.JobOptions{}), handle)
		})
	}
}

func TestOnEveryEngineAJobRoundTripsThroughItsStates(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, handle *gorm.DB) {
		id, err := jobs.Enqueue(t.Context(), queue, "email.welcome", welcomeArgs{UserID: "u1"})
		if err != nil {
			t.Fatalf("enqueueing: %v", err)
		}

		claimed, err := queue.Claim(t.Context(), "worker-1", nil, 1)
		if err != nil {
			t.Fatalf("claiming: %v", err)
		}
		if len(claimed) != 1 || claimed[0].ID != id || claimed[0].Attempts != 1 {
			t.Fatalf("claimed %+v", claimed)
		}
		if string(claimed[0].Payload) != `{"user_id":"u1","tries":0}` {
			t.Fatalf("the payload came back as %s", claimed[0].Payload)
		}

		if err := queue.Complete(t.Context(), id, time.Now()); err != nil {
			t.Fatalf("completing: %v", err)
		}
		if stored := storedJob(t, handle, id); stored.State != jobs.Done || stored.FinishedAt == nil {
			t.Fatalf("the finished job is %+v", stored)
		}
	})
}

func TestOnEveryEngineOneJobIsClaimedByExactlyOneWorker(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, _ *gorm.DB) {
		const total = 5
		for i := range total {
			if _, err := jobs.Enqueue(t.Context(), queue, "contended."+strconv.Itoa(i), nil); err != nil {
				t.Fatalf("enqueueing: %v", err)
			}
		}

		var (
			mutex   sync.Mutex
			claimed []string
			wait    sync.WaitGroup
		)
		for worker := range 10 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				taken, err := queue.Claim(context.Background(), "worker-"+strconv.Itoa(worker), nil, 1)
				if err != nil {
					return
				}
				mutex.Lock()
				defer mutex.Unlock()
				for _, record := range taken {
					claimed = append(claimed, record.ID)
				}
			}()
		}
		wait.Wait()

		if len(claimed) != total {
			t.Fatalf("10 workers claimed %d of %d jobs", len(claimed), total)
		}
		seen := map[string]bool{}
		for _, id := range claimed {
			if seen[id] {
				t.Fatalf("job %s was claimed twice", id)
			}
			seen[id] = true
		}
	})
}

func TestOnEveryEngineRunAtIsComparedAsATime(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, _ *gorm.DB) {
		if _, err := jobs.Enqueue(t.Context(), queue, "later", nil, jobs.Options{Delay: time.Hour}); err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
		ready, err := jobs.Enqueue(t.Context(), queue, "now", nil)
		if err != nil {
			t.Fatalf("enqueueing: %v", err)
		}

		claimed, err := queue.Claim(t.Context(), "worker-1", nil, 5)
		if err != nil {
			t.Fatalf("claiming: %v", err)
		}
		if len(claimed) != 1 || claimed[0].ID != ready {
			t.Fatalf("claimed %d jobs; a job an hour out must not be due", len(claimed))
		}
	})
}

func TestOnEveryEngineAnAbandonedLockIsReclaimedByTime(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, handle *gorm.DB) {
		id, err := jobs.Enqueue(t.Context(), queue, "orphan", nil, jobs.Options{MaxAttempts: 5})
		if err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
		if _, err := queue.Claim(t.Context(), "a-worker-that-died", nil, 1); err != nil {
			t.Fatalf("claiming: %v", err)
		}

		fresh, err := queue.Recover(t.Context(), time.Now().Add(-time.Hour))
		if err != nil {
			t.Fatalf("recovering: %v", err)
		}
		if fresh != 0 {
			t.Fatalf("recovered %d jobs while the lock was fresh", fresh)
		}

		stale := time.Now().Add(-time.Hour).UTC()
		if err := handle.Model(&jobs.Record{}).Where("id = ?", id).Update("locked_at", stale).Error; err != nil {
			t.Fatalf("ageing the lock: %v", err)
		}
		recovered, err := queue.Recover(t.Context(), time.Now().Add(-time.Minute))
		if err != nil {
			t.Fatalf("recovering: %v", err)
		}
		if recovered != 1 {
			t.Fatalf("recovered %d jobs, want 1", recovered)
		}
		if stored := storedJob(t, handle, id); stored.State != jobs.Queued {
			t.Fatalf("the reclaimed job is %q", stored.State)
		}
	})
}

func TestOnEveryEngineManyJobsWithNoFingerprintCoexist(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, handle *gorm.DB) {
		for i := range 5 {
			if _, err := jobs.Enqueue(t.Context(), queue, "unmarked."+strconv.Itoa(i), nil); err != nil {
				t.Fatalf("enqueueing: %v", err)
			}
		}

		var total int64
		if err := handle.Model(&jobs.Record{}).Count(&total).Error; err != nil {
			t.Fatalf("counting: %v", err)
		}
		if total != 5 {
			t.Fatalf("%d rows were stored; an unset fingerprint must be NULL, not an empty string", total)
		}
	})
}

func TestOnEveryEngineADuplicateFingerprintIsRefused(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, _ *gorm.DB) {
		options := jobs.Options{Fingerprint: "uploads.sweep@2026-09-11T14:00:00Z"}
		if _, err := jobs.Enqueue(t.Context(), queue, "uploads.sweep", nil, options); err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
		_, err := jobs.Enqueue(t.Context(), queue, "uploads.sweep", nil, options)
		if !errors.Is(err, jobs.ErrDuplicateFingerprint) {
			t.Fatalf("got %v, want ErrDuplicateFingerprint", err)
		}
	})
}

func TestOnEveryEngineOnlyAFinishedJobCanBeRetried(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, handle *gorm.DB) {
		id, err := jobs.Enqueue(t.Context(), queue, "doomed", nil, jobs.Options{MaxAttempts: 1})
		if err != nil {
			t.Fatalf("enqueueing: %v", err)
		}

		if err := queue.Retry(t.Context(), id, time.Now()); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("retrying a queued job returned %v, want ErrNotFound", err)
		}
		if _, err := queue.Claim(t.Context(), "worker-1", nil, 1); err != nil {
			t.Fatalf("claiming: %v", err)
		}
		if err := queue.Retry(t.Context(), id, time.Now()); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("retrying a running job returned %v, want ErrNotFound", err)
		}

		if err := queue.Fail(t.Context(), id, "gave up", nil); err != nil {
			t.Fatalf("failing: %v", err)
		}
		if err := queue.Retry(t.Context(), id, time.Now()); err != nil {
			t.Fatalf("retrying a dead job: %v", err)
		}
		stored := storedJob(t, handle, id)
		if stored.State != jobs.Queued || stored.Attempts != 0 || stored.LastError != "" {
			t.Fatalf("the retried job is %+v", stored)
		}
	})
}

func TestOnEveryEngineStatsAndSweepAgree(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, handle *gorm.DB) {
		done, err := jobs.Enqueue(t.Context(), queue, "finished", nil, jobs.Options{Priority: 9})
		if err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
		if _, err := jobs.Enqueue(t.Context(), queue, "waiting", nil); err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
		if _, err := queue.Claim(t.Context(), "worker-1", nil, 1); err != nil {
			t.Fatalf("claiming: %v", err)
		}
		if err := queue.Complete(t.Context(), done, time.Now()); err != nil {
			t.Fatalf("completing: %v", err)
		}

		stats, err := queue.Stats(t.Context())
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.Done != 1 || stats.Queued != 1 || stats.Total() != 2 {
			t.Fatalf("stats are %+v", stats)
		}

		removed, err := queue.Sweep(t.Context(), time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("sweeping: %v", err)
		}
		if removed != 1 {
			t.Fatalf("swept %d rows, want only the finished one", removed)
		}
		var remaining int64
		if err := handle.Model(&jobs.Record{}).Count(&remaining).Error; err != nil {
			t.Fatalf("counting: %v", err)
		}
		if remaining != 1 {
			t.Fatalf("%d rows remain, want the queued one", remaining)
		}
	})
}

func TestOnEveryEngineAJobRollsBackWithItsTransaction(t *testing.T) {
	onEachEngine(t, func(t *testing.T, queue jobs.Queue, handle *gorm.DB) {
		refused := errors.New("no")
		err := handle.Transaction(func(tx *gorm.DB) error {
			if _, err := jobs.Enqueue(t.Context(), queue.WithTx(tx), "email.welcome", nil); err != nil {
				return err
			}
			return refused
		})
		if !errors.Is(err, refused) {
			t.Fatalf("the transaction returned %v", err)
		}

		var total int64
		if err := handle.Model(&jobs.Record{}).Count(&total).Error; err != nil {
			t.Fatalf("counting: %v", err)
		}
		if total != 0 {
			t.Fatalf("%d jobs survived a rolled-back transaction", total)
		}
	})
}
