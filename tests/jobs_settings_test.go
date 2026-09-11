package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/settings"
)

func jobSettings(t *testing.T, fns ...func(*settings.Settings)) (settings.Settings, error) {
	t.Helper()
	base := devSettings(t, func(s *settings.Settings) {
		s.Jobs.Enabled = true
	})
	for _, fn := range fns {
		fn(&base)
	}
	return settings.New(func(s *settings.Settings) { *s = base })
}

func refusedBecause(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil {
		t.Fatalf("the configuration was accepted; it should have been refused for %q", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("refused with %q, which does not mention %q", err.Error(), fragment)
	}
}

func TestJobsAreOffByDefaultAndCostNothing(t *testing.T) {
	defaults := settings.Default()
	if defaults.Jobs.Enabled {
		t.Fatal("Jobs.Enabled must default to false")
	}
	if defaults.Jobs.Active() || defaults.Jobs.RunsWorkers() {
		t.Fatal("a default configuration reports jobs as active")
	}
	if defaults.Jobs.Workers != 0 {
		t.Fatalf("Jobs.Workers defaults to %d, want 0 so a web process never becomes a worker", defaults.Jobs.Workers)
	}

	a := newTestApp(t)
	for _, m := range a.Models() {
		if _, isJob := m.Entity.(jobs.Record); isJob {
			t.Fatal("the jobs table is registered while jobs are off, so makemigrations would create it")
		}
	}
}

func TestEnablingJobsRegistersTheTable(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) { s.Jobs.Enabled = true })

	found := false
	for _, m := range a.Models() {
		if _, isJob := m.Entity.(jobs.Record); isJob {
			found = true
		}
	}
	if !found {
		t.Fatal("jobs are on but the jobs table is not registered, so makemigrations would never create it")
	}
}

func TestTheJobDefaultsAreUsable(t *testing.T) {
	defaults := settings.Default().Jobs

	if defaults.MaxAttempts != jobs.DefaultMaxAttempts {
		t.Fatalf("MaxAttempts defaults to %d", defaults.MaxAttempts)
	}
	if defaults.ClaimTimeout <= defaults.DrainTimeout {
		t.Fatalf("ClaimTimeout %v must exceed DrainTimeout %v", defaults.ClaimTimeout, defaults.DrainTimeout)
	}
	if defaults.Backoff <= 0 || defaults.BackoffCeiling < defaults.Backoff {
		t.Fatalf("backoff defaults are %v and %v", defaults.Backoff, defaults.BackoffCeiling)
	}
	if defaults.PollInterval <= 0 {
		t.Fatalf("PollInterval defaults to %v", defaults.PollInterval)
	}
	if len(defaults.QueueNames()) != 1 || defaults.QueueNames()[0] != jobs.DefaultQueue {
		t.Fatalf("QueueNames() is %v", defaults.QueueNames())
	}
}

func TestTheDefaultJobConfigurationValidates(t *testing.T) {
	if _, err := jobSettings(t); err != nil {
		t.Fatalf("turning jobs on with everything else defaulted was refused: %v", err)
	}
}

func TestAWorkerWithoutJobsEnabledIsRefused(t *testing.T) {
	_, err := settings.New(func(s *settings.Settings) {
		*s = devSettings(t)
		s.Jobs.Enabled = false
		s.Jobs.Workers = 4
	})
	refusedBecause(t, err, "Jobs.Workers is set but Jobs.Enabled is false")
}

func TestACustomQueueWithoutJobsEnabledIsRefused(t *testing.T) {
	_, err := settings.New(func(s *settings.Settings) {
		*s = devSettings(t)
		s.Jobs.Enabled = false
		s.Jobs.Queue = &recordingQueue{}
	})
	refusedBecause(t, err, "Jobs.Queue is set but Jobs.Enabled is false")
}

func TestNegativeWorkersAreRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.Workers = -1 })
	refusedBecause(t, err, "Jobs.Workers cannot be negative")
}

func TestAnEmptyQueueListIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.Queues = []string{} })
	refusedBecause(t, err, "Jobs.Queues is empty")
}

func TestABlankQueueNameIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.Queues = []string{"default", "  "} })
	refusedBecause(t, err, "Jobs.Queues[1] is empty")
}

func TestZeroMaxAttemptsIsRefusedBecauseItWouldBuryEveryJob(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.MaxAttempts = 0 })
	refusedBecause(t, err, "would bury every job on its first failure")
}

func TestForeverIsAnAcceptableAttemptCeiling(t *testing.T) {
	if _, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.MaxAttempts = jobs.Forever }); err != nil {
		t.Fatalf("jobs.Forever was refused as a ceiling: %v", err)
	}
}

func TestAnArbitraryNegativeAttemptCeilingIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.MaxAttempts = -7 })
	refusedBecause(t, err, "is not a count")
}

func TestACeilingShorterThanTheFirstBackoffIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) {
		s.Jobs.Backoff = time.Minute
		s.Jobs.BackoffCeiling = time.Second
	})
	refusedBecause(t, err, "the very first retry would already be capped")
}

func TestAClaimTimeoutInsideTheDrainWindowIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) {
		s.Jobs.DrainTimeout = time.Minute
		s.Jobs.ClaimTimeout = 30 * time.Second
	})
	refusedBecause(t, err, "reclaimed and run twice")
}

func TestABusyPollIntervalIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.PollInterval = 5 * time.Millisecond })
	refusedBecause(t, err, "busy loop against the database")
}

func TestAnUnsetPollIntervalIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.PollInterval = 0 })
	refusedBecause(t, err, "Jobs.PollInterval must be greater than zero")
}

func TestANegativeDoneTTLIsRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Jobs.DoneTTL = -time.Hour })
	refusedBecause(t, err, "Jobs.DoneTTL cannot be negative")
}

func TestJobsWithoutADatabaseOrAQueueAreRefused(t *testing.T) {
	_, err := jobSettings(t, func(s *settings.Settings) { s.Databases = nil })
	refusedBecause(t, err, "the queue is a table")
}

func TestACustomQueueStandsInForTheDatabase(t *testing.T) {
	if _, err := jobSettings(t, func(s *settings.Settings) {
		s.Databases = nil
		s.Jobs.Queue = &recordingQueue{}
	}); err != nil {
		if strings.Contains(err.Error(), "the queue is a table") {
			t.Fatalf("a supplied Jobs.Queue should remove the need for a database: %v", err)
		}
	}
}
