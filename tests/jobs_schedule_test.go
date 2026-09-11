package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/settings"
)

func TestAScheduleRefusesAnEntryWithNoIntervalOrKind(t *testing.T) {
	schedule := jobs.NewSchedule()

	if err := schedule.Add(jobs.Periodic{Kind: "uploads.sweep"}); !errors.Is(err, jobs.ErrIntervalMissing) {
		t.Fatalf("got %v, want ErrIntervalMissing", err)
	}
	if err := schedule.Add(jobs.Periodic{Every: time.Hour}); !errors.Is(err, jobs.ErrKindMissing) {
		t.Fatalf("got %v, want ErrKindMissing", err)
	}
	if schedule.Len() != 0 {
		t.Fatal("a rejected entry still landed in the schedule")
	}
}

func TestAScheduleRefusesTheSameKindTwice(t *testing.T) {
	schedule := jobs.NewSchedule()
	entry := jobs.Periodic{Kind: "uploads.sweep", Every: time.Hour}

	if err := schedule.Add(entry); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := schedule.Add(entry); !errors.Is(err, jobs.ErrDuplicateSchedule) {
		t.Fatalf("got %v, want ErrDuplicateSchedule", err)
	}
}

func TestEntriesComeBackInOrderAcrossAParentSchedule(t *testing.T) {
	parent := jobs.NewSchedule()
	if err := parent.Add(jobs.Periodic{Kind: "reports.nightly", Every: time.Hour}); err != nil {
		t.Fatalf("adding to the parent: %v", err)
	}

	child := jobs.ScheduleUnder(parent)
	if err := child.Add(jobs.Periodic{Kind: "cache.warm", Every: time.Minute}); err != nil {
		t.Fatalf("adding to the child: %v", err)
	}

	entries := child.Entries()
	if len(entries) != 2 || entries[0].Kind != "cache.warm" || entries[1].Kind != "reports.nightly" {
		t.Fatalf("entries are %+v", entries)
	}
	if !child.Holds("reports.nightly") {
		t.Fatal("the child does not see the parent's entry")
	}
	if err := child.Add(jobs.Periodic{Kind: "reports.nightly", Every: time.Minute}); !errors.Is(err, jobs.ErrDuplicateSchedule) {
		t.Fatalf("shadowing a parent entry returned %v, want ErrDuplicateSchedule", err)
	}
}

func TestASlotIsStableWithinItsIntervalAndMovesAfterIt(t *testing.T) {
	entry := jobs.Periodic{Kind: "uploads.sweep", Every: time.Hour}
	base := time.Date(2026, 9, 11, 14, 0, 0, 0, time.UTC)

	if entry.Slot(base) != entry.Slot(base.Add(59*time.Minute)) {
		t.Fatal("two times in the same hour produced different slots")
	}
	if entry.Slot(base) == entry.Slot(base.Add(time.Hour)) {
		t.Fatal("the next hour produced the same slot")
	}
	if entry.Fingerprint(entry.Slot(base)) == entry.Fingerprint(entry.Slot(base.Add(time.Hour))) {
		t.Fatal("two slots produced the same fingerprint")
	}
}

func TestAFingerprintFitsTheColumn(t *testing.T) {
	entry := jobs.Periodic{Kind: "a.very.long.job.kind.that.somebody.might.plausibly.write.out.in.full", Every: time.Minute}
	if size := len(entry.Fingerprint(entry.Slot(time.Now()))); size > 200 {
		t.Fatalf("a fingerprint is %d characters, past the 200 the column allows", size)
	}
}

func TestARegistryUnderAParentFindsBothAndRefusesShadowing(t *testing.T) {
	parent := jobs.NewRegistry()
	if err := jobs.HandleOn(parent, "email.welcome", func(_ context.Context, _ welcomeArgs) error { return nil }); err != nil {
		t.Fatalf("registering on the parent: %v", err)
	}

	child := jobs.RegistryUnder(parent)
	if err := jobs.HandleOn(child, "cache.warm", func(_ context.Context, _ welcomeArgs) error { return nil }); err != nil {
		t.Fatalf("registering on the child: %v", err)
	}

	if !child.Knows("email.welcome") || !child.Knows("cache.warm") {
		t.Fatalf("the child sees %v", child.Kinds())
	}
	if err := child.Run(context.Background(), "email.welcome", nil); err != nil {
		t.Fatalf("running the parent's handler through the child: %v", err)
	}

	err := jobs.HandleOn(child, "email.welcome", func(_ context.Context, _ welcomeArgs) error { return nil })
	if !errors.Is(err, jobs.ErrDuplicateKind) {
		t.Fatalf("shadowing a parent handler returned %v, want ErrDuplicateKind", err)
	}

	kinds := child.Kinds()
	if len(kinds) != 2 || kinds[0] != "cache.warm" || kinds[1] != "email.welcome" {
		t.Fatalf("Kinds() is %v", kinds)
	}
}

func TestAPeriodicJobIsEnqueuedOncePerSlot(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	release := make(chan struct{})
	if err := jobs.HandleOn(registry, "cache.warm", func(_ context.Context, _ struct{}) error {
		<-release
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	schedule := jobs.NewSchedule()
	if err := schedule.Add(jobs.Periodic{Kind: "cache.warm", Every: time.Hour}); err != nil {
		t.Fatalf("scheduling: %v", err)
	}

	runner := newRunner(t, queue, registry, func(o *jobs.RunnerOptions) {
		o.Schedule = schedule
		o.Workers = 2
	})
	runner.Start()

	waitFor(t, "the periodic job to appear", func() bool {
		var total int64
		if err := handle.Model(&jobs.Record{}).Where("kind = ?", "cache.warm").Count(&total).Error; err != nil {
			return false
		}
		return total >= 1
	})
	time.Sleep(120 * time.Millisecond)

	var total int64
	if err := handle.Model(&jobs.Record{}).Where("kind = ?", "cache.warm").Count(&total).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if total != 1 {
		t.Fatalf("%d copies of an hourly job were enqueued inside one slot, want 1", total)
	}
	close(release)
}

func TestTwoRunnersSharingADatabaseStillEnqueueOneCopy(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	release := make(chan struct{})
	if err := jobs.HandleOn(registry, "cache.warm", func(_ context.Context, _ struct{}) error {
		<-release
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	tune := func(o *jobs.RunnerOptions) {
		schedule := jobs.NewSchedule()
		if err := schedule.Add(jobs.Periodic{Kind: "cache.warm", Every: time.Hour}); err != nil {
			t.Fatalf("scheduling: %v", err)
		}
		o.Schedule = schedule
	}
	newRunner(t, queue, registry, tune).Start()
	newRunner(t, queue, registry, tune).Start()

	waitFor(t, "the periodic job to appear", func() bool {
		var total int64
		if err := handle.Model(&jobs.Record{}).Where("kind = ?", "cache.warm").Count(&total).Error; err != nil {
			return false
		}
		return total >= 1
	})
	time.Sleep(150 * time.Millisecond)

	var total int64
	if err := handle.Model(&jobs.Record{}).Where("kind = ?", "cache.warm").Count(&total).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if total != 1 {
		t.Fatalf("two runners enqueued %d copies of the same slot, want 1", total)
	}
	close(release)
}

func TestTheUploadSweepIsScheduledWhenUploadsAndJobsAreBothOn(t *testing.T) {
	a := newJobApp(t, func(s *settings.Settings) {
		s.Jobs.Workers = 1
		s.Uploads.Enabled = true
		s.Uploads.Dir = t.TempDir()
	})

	runner := a.Jobs()
	if runner == nil {
		t.Fatal("workers are configured but the app has no runner")
	}
	if a.Uploads == nil {
		t.Fatal("uploads are on but the app has no upload service")
	}
}

func TestNoRunnerExistsWithoutWorkers(t *testing.T) {
	a := newJobApp(t)
	if a.Jobs() != nil {
		t.Fatal("a runner exists while Jobs.Workers is 0")
	}
}

func TestTheBuiltInSweepKindsAreNamespaced(t *testing.T) {
	for _, kind := range []string{app.SweepUploads, app.SweepTokens} {
		if len(kind) == 0 || kind[:7] != "coyote." {
			t.Fatalf("the built-in kind %q is not namespaced, so it could collide with a user's job", kind)
		}
	}
}

func TestASweepJobRunsThroughTheRunner(t *testing.T) {
	a := newJobApp(t, func(s *settings.Settings) {
		s.Jobs.Workers = 1
		s.Jobs.PollInterval = 100 * time.Millisecond
		s.Uploads.Enabled = true
		s.Uploads.Dir = t.TempDir()
	})

	runner := a.Jobs()
	if runner == nil {
		t.Fatal("no runner")
	}
	runner.Start()
	t.Cleanup(func() { _ = runner.Close() })

	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	waitFor(t, "the built-in upload sweep to be enqueued and finish", func() bool {
		var total int64
		if err := handle.Model(&jobs.Record{}).
			Where("kind = ? AND state = ?", app.SweepUploads, jobs.Done).
			Count(&total).Error; err != nil {
			return false
		}
		return total == 1
	})
}
