package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

func newJobApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	a := newTestApp(t, append([]func(*settings.Settings){
		func(s *settings.Settings) { s.Jobs.Enabled = true },
	}, fns...)...)
	t.Cleanup(func() { _ = a.CloseDB() })
	return a
}

func countJobs(t *testing.T, a *app.App) int64 {
	t.Helper()
	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	var total int64
	if err := handle.Model(&jobs.Record{}).Count(&total).Error; err != nil {
		t.Fatalf("counting jobs: %v", err)
	}
	return total
}

func TestTheAppHasNoQueueWhileJobsAreOff(t *testing.T) {
	a := newTestApp(t)
	if a.Queue() != nil {
		t.Fatal("a queue exists while jobs are off")
	}

	_, err := jobs.Enqueue(context.Background(), a.Queue(), "email.welcome", nil)
	if !errors.Is(err, jobs.ErrNoQueue) {
		t.Fatalf("enqueueing with jobs off returned %v, want ErrNoQueue", err)
	}
}

func TestTheAppQueueEnqueuesWhenJobsAreOn(t *testing.T) {
	a := newJobApp(t)
	if a.Queue() == nil {
		t.Fatal("jobs are on but the app has no queue")
	}

	id, err := jobs.Enqueue(context.Background(), a.Queue(), "email.welcome", welcomeArgs{UserID: "u1"})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if id == "" {
		t.Fatal("the queue returned no job id")
	}
	if countJobs(t, a) != 1 {
		t.Fatalf("%d jobs were stored, want 1", countJobs(t, a))
	}
}

func TestASuppliedQueueReplacesTheDatabaseOne(t *testing.T) {
	recorder := &recordingQueue{}
	a := newJobApp(t, func(s *settings.Settings) { s.Jobs.Queue = recorder })

	if _, err := jobs.Enqueue(context.Background(), a.Queue(), "email.welcome", nil); err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("the supplied queue saw %d enqueues, want 1", recorder.calls)
	}
	if countJobs(t, a) != 0 {
		t.Fatal("the job reached the database even though a queue was supplied")
	}
}

func TestTheAppQueueAppliesTheConfiguredAttemptCeiling(t *testing.T) {
	a := newJobApp(t, func(s *settings.Settings) { s.Jobs.MaxAttempts = 11 })

	id, err := jobs.Enqueue(context.Background(), a.Queue(), "email.welcome", nil)
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	if stored := storedJob(t, handle, id); stored.MaxAttempts != 11 {
		t.Fatalf("max_attempts is %d, want the configured 11", stored.MaxAttempts)
	}
}

func TestARolledBackTransactionTakesItsJobWithIt(t *testing.T) {
	a := newJobApp(t)
	refused := errors.New("the record did not validate")

	err := a.Transaction(context.Background(), func(tx *app.Tx) error {
		if _, err := jobs.Enqueue(context.Background(), tx.Queue(), "email.welcome", nil); err != nil {
			return err
		}
		return refused
	})
	if !errors.Is(err, refused) {
		t.Fatalf("the transaction returned %v", err)
	}
	if total := countJobs(t, a); total != 0 {
		t.Fatalf("%d jobs survived a rolled-back transaction", total)
	}
}

func TestAJobAndTheRecordItIsAboutCommitTogether(t *testing.T) {
	a := newJobApp(t)

	schema, err := a.Describe(jobs.Record{})
	if err != nil {
		t.Fatalf("describing: %v", err)
	}
	if schema == nil {
		t.Fatal("no schema")
	}

	var enqueued string
	err = a.Transaction(context.Background(), func(tx *app.Tx) error {
		if tx.Store() == nil {
			t.Fatal("the transaction carries no store")
		}
		id, err := jobs.Enqueue(context.Background(), tx.Queue(), "email.welcome", welcomeArgs{UserID: "u1"})
		enqueued = id
		return err
	})
	if err != nil {
		t.Fatalf("the transaction failed: %v", err)
	}

	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	stored := storedJob(t, handle, enqueued)
	if stored.State != jobs.Queued || stored.Kind != "email.welcome" {
		t.Fatalf("the committed job looks like %+v", stored)
	}
}

func TestTheTransactionCarriesNoQueueWhileJobsAreOff(t *testing.T) {
	a := newTestApp(t)

	err := a.Transaction(context.Background(), func(tx *app.Tx) error {
		if tx.Queue() != nil {
			t.Fatal("the transaction carries a queue while jobs are off")
		}
		_, err := jobs.Enqueue(context.Background(), tx.Queue(), "email.welcome", nil)
		return err
	})
	if !errors.Is(err, jobs.ErrNoQueue) {
		t.Fatalf("the transaction returned %v, want ErrNoQueue", err)
	}
}

func TestAJobEnqueuedOutsideATransactionIsNotRolledBackByALaterOne(t *testing.T) {
	a := newJobApp(t)
	refused := errors.New("no")

	if _, err := jobs.Enqueue(context.Background(), a.Queue(), "email.welcome", nil); err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	_ = a.Transaction(context.Background(), func(tx *app.Tx) error { return refused })

	if total := countJobs(t, a); total != 1 {
		t.Fatalf("%d jobs remain, want the one enqueued outside the transaction", total)
	}
}

func TestTheJobsTableIsPartOfTheAppsModelsWhenJobsAreOn(t *testing.T) {
	a := newJobApp(t)

	schema, err := a.Describe(jobs.Record{})
	if err != nil {
		t.Fatalf("describing: %v", err)
	}
	records, err := a.Store()
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}

	total, err := records.Count(context.Background(), schema, model.Query{})
	if err != nil {
		t.Fatalf("the generic store cannot read the jobs table: %v", err)
	}
	if total != 0 {
		t.Fatalf("a fresh app has %d jobs", total)
	}
}
