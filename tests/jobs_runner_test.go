package tests

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newRunner(t *testing.T, queue jobs.Queue, registry *jobs.Registry, tune ...func(*jobs.RunnerOptions)) *jobs.Runner {
	t.Helper()
	opts := jobs.RunnerOptions{
		Queue:        queue,
		Registry:     registry,
		Name:         "test",
		Workers:      1,
		PollInterval: 5 * time.Millisecond,
		ClaimTimeout: time.Minute,
		DrainTimeout: 2 * time.Second,
		Backoff:      time.Millisecond,
		Ceiling:      10 * time.Millisecond,
		Logger:       quietLogger(),
	}
	for _, fn := range tune {
		fn(&opts)
	}
	runner := jobs.NewRunner(opts)
	t.Cleanup(func() { _ = runner.Close() })
	return runner
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestARunnerRunsAQueuedJobAndMarksItDone(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	var seen atomic.Value
	if err := jobs.HandleOn(registry, "email.welcome", func(_ context.Context, args welcomeArgs) error {
		seen.Store(args.UserID)
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "email.welcome", welcomeArgs{UserID: "u1"})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	newRunner(t, queue, registry).Start()
	waitFor(t, "the job to finish", func() bool {
		return storedJob(t, handle, id).State == jobs.Done
	})
	if seen.Load() != "u1" {
		t.Fatalf("the handler saw %v", seen.Load())
	}

	stored := storedJob(t, handle, id)
	if stored.FinishedAt == nil || stored.LockedBy != "" || stored.Attempts != 1 {
		t.Fatalf("the finished job looks like %+v", stored)
	}
}

func TestAFailingJobIsRetriedUntilItSucceeds(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	var attempts atomic.Int64
	if err := jobs.HandleOn(registry, "flaky", func(_ context.Context, _ welcomeArgs) error {
		if attempts.Add(1) < 3 {
			return errors.New("not yet")
		}
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "flaky", nil, jobs.Options{MaxAttempts: 5})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	newRunner(t, queue, registry).Start()
	waitFor(t, "the third attempt to succeed", func() bool {
		return storedJob(t, handle, id).State == jobs.Done
	})

	stored := storedJob(t, handle, id)
	if stored.Attempts != 3 {
		t.Fatalf("the job took %d attempts, want 3", stored.Attempts)
	}
	if attempts.Load() != 3 {
		t.Fatalf("the handler ran %d times", attempts.Load())
	}
}

func TestAJobThatKeepsFailingEndsUpDeadWithItsReason(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	if err := jobs.HandleOn(registry, "doomed", func(_ context.Context, _ welcomeArgs) error {
		return errors.New("the remote host refused")
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "doomed", nil, jobs.Options{MaxAttempts: 2})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	newRunner(t, queue, registry).Start()
	waitFor(t, "the job to be buried", func() bool {
		return storedJob(t, handle, id).State == jobs.Dead
	})

	stored := storedJob(t, handle, id)
	if stored.Attempts != 2 {
		t.Fatalf("the job was buried after %d attempts, want 2", stored.Attempts)
	}
	if stored.LastError != "the remote host refused" {
		t.Fatalf("last_error is %q", stored.LastError)
	}
	if stored.FinishedAt == nil {
		t.Fatal("a buried job has no finished_at")
	}
}

func TestAPanickingJobIsRecordedAndTheWorkerKeepsGoing(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	if err := jobs.HandleOn(registry, "explodes", func(_ context.Context, _ welcomeArgs) error {
		panic("the pointer was nil")
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}
	var survived atomic.Bool
	if err := jobs.HandleOn(registry, "survives", func(_ context.Context, _ welcomeArgs) error {
		survived.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	bad, err := jobs.Enqueue(context.Background(), queue, "explodes", nil, jobs.Options{MaxAttempts: 1, Priority: 9})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if _, err := jobs.Enqueue(context.Background(), queue, "survives", nil); err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	newRunner(t, queue, registry).Start()
	waitFor(t, "the panicking job to be buried", func() bool {
		return storedJob(t, handle, bad).State == jobs.Dead
	})
	waitFor(t, "the next job to run anyway", func() bool { return survived.Load() })

	if stored := storedJob(t, handle, bad); stored.LastError == "" {
		t.Fatal("a panicking job recorded no reason")
	}
}

func TestAnUnknownKindIsBuriedWithoutBurningRetries(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	id, err := jobs.Enqueue(context.Background(), queue, "nobody.handles.this", nil, jobs.Options{MaxAttempts: 9})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	newRunner(t, queue, registry).Start()
	waitFor(t, "the unhandled job to be buried", func() bool {
		return storedJob(t, handle, id).State == jobs.Dead
	})

	stored := storedJob(t, handle, id)
	if stored.Attempts != 1 {
		t.Fatalf("an unhandled job used %d of its 9 attempts, want 1", stored.Attempts)
	}
}

func TestAPayloadThatNoLongerDecodesIsBuriedWithoutBurningRetries(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	if err := jobs.HandleOn(registry, "renamed", func(_ context.Context, _ welcomeArgs) error {
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "renamed", map[string]any{"user_id": []int{1, 2}},
		jobs.Options{MaxAttempts: 9})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	newRunner(t, queue, registry).Start()
	waitFor(t, "the undecodable job to be buried", func() bool {
		return storedJob(t, handle, id).State == jobs.Dead
	})

	if stored := storedJob(t, handle, id); stored.Attempts != 1 {
		t.Fatalf("an undecodable job used %d of its 9 attempts, want 1", stored.Attempts)
	}
}

func TestManyWorkersRunEveryJobExactlyOnce(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	var (
		mutex sync.Mutex
		ran   = map[string]int{}
	)
	if err := jobs.HandleOn(registry, "counted", func(_ context.Context, args welcomeArgs) error {
		mutex.Lock()
		defer mutex.Unlock()
		ran[args.UserID]++
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	const total = 25
	for i := range total {
		if _, err := jobs.Enqueue(context.Background(), queue, "counted",
			welcomeArgs{UserID: strconv.Itoa(i)}); err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
	}

	newRunner(t, queue, registry, func(o *jobs.RunnerOptions) { o.Workers = 6 }).Start()

	waitFor(t, "every job to finish", func() bool {
		var done int64
		if err := handle.Model(&jobs.Record{}).Where("state = ?", jobs.Done).Count(&done).Error; err != nil {
			return false
		}
		return done == total
	})

	mutex.Lock()
	defer mutex.Unlock()
	if len(ran) != total {
		t.Fatalf("%d distinct jobs ran, want %d", len(ran), total)
	}
	for id, times := range ran {
		if times != 1 {
			t.Fatalf("job %s ran %d times", id, times)
		}
	}
}

func TestARunnerDrainsAJobAlreadyInFlightBeforeItStops(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	entered := make(chan struct{})
	var finished atomic.Bool
	if err := jobs.HandleOn(registry, "slow", func(_ context.Context, _ welcomeArgs) error {
		close(entered)
		time.Sleep(250 * time.Millisecond)
		finished.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "slow", nil)
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	runner := newRunner(t, queue, registry)
	runner.Start()
	<-entered

	if err := runner.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if !finished.Load() {
		t.Fatal("Close returned before the job in flight had finished")
	}
	if stored := storedJob(t, handle, id); stored.State != jobs.Done {
		t.Fatalf("the drained job is %q, want done", stored.State)
	}
}

func TestAJobThatOutlastsTheDrainWindowIsCancelledAndRequeued(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	entered := make(chan struct{})
	if err := jobs.HandleOn(registry, "stubborn", func(ctx context.Context, _ welcomeArgs) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "stubborn", nil, jobs.Options{MaxAttempts: 5})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	runner := newRunner(t, queue, registry, func(o *jobs.RunnerOptions) {
		o.DrainTimeout = 50 * time.Millisecond
	})
	runner.Start()
	<-entered

	if err := runner.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	waitFor(t, "the cancelled job to go back in the queue", func() bool {
		return storedJob(t, handle, id).State == jobs.Queued
	})
}

func TestALongJobHeartbeatsSoItIsNotReclaimed(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	release := make(chan struct{})
	if err := jobs.HandleOn(registry, "lengthy", func(_ context.Context, _ welcomeArgs) error {
		<-release
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "lengthy", nil)
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	runner := newRunner(t, queue, registry, func(o *jobs.RunnerOptions) {
		o.ClaimTimeout = 60 * time.Millisecond
	})
	runner.Start()

	waitFor(t, "the job to start", func() bool {
		return storedJob(t, handle, id).State == jobs.Running
	})
	first := storedJob(t, handle, id)

	waitFor(t, "the lock to be pushed forward", func() bool {
		current := storedJob(t, handle, id)
		return current.LockedAt != nil && current.LockedAt.After(*first.LockedAt)
	})
	if stored := storedJob(t, handle, id); stored.State != jobs.Running {
		t.Fatalf("a heartbeating job was reclaimed anyway: %q", stored.State)
	}

	close(release)
	waitFor(t, "the job to finish", func() bool {
		return storedJob(t, handle, id).State == jobs.Done
	})
}

func TestTheRunnerReclaimsAJobAbandonedByAnotherProcess(t *testing.T) {
	queue, handle := newJobQueue(t)
	registry := jobs.NewRegistry()

	var ran atomic.Bool
	if err := jobs.HandleOn(registry, "orphan", func(_ context.Context, _ welcomeArgs) error {
		ran.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	id, err := jobs.Enqueue(context.Background(), queue, "orphan", nil, jobs.Options{MaxAttempts: 5})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if _, err := queue.Claim(context.Background(), "a-worker-that-died", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	stale := time.Now().Add(-time.Hour).UTC()
	if err := handle.Model(&jobs.Record{}).Where("id = ?", id).
		Update("locked_at", stale).Error; err != nil {
		t.Fatalf("ageing the lock: %v", err)
	}

	newRunner(t, queue, registry, func(o *jobs.RunnerOptions) {
		o.ClaimTimeout = 20 * time.Millisecond
	}).Start()

	waitFor(t, "the orphaned job to be picked up and finished", func() bool {
		return storedJob(t, handle, id).State == jobs.Done
	})
	if !ran.Load() {
		t.Fatal("the orphaned job was marked done without running")
	}
}

func TestClosingARunnerTwiceIsSafe(t *testing.T) {
	queue, _ := newJobQueue(t)
	runner := newRunner(t, queue, jobs.NewRegistry())
	runner.Start()

	if err := runner.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestARunnerWithNoQueueDoesNothing(t *testing.T) {
	runner := jobs.NewRunner(jobs.RunnerOptions{Registry: jobs.NewRegistry(), Logger: quietLogger()})
	runner.Start()
	if err := runner.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
}

func TestBackoffGrowsAndRespectsItsCeiling(t *testing.T) {
	base, ceiling := time.Second, 10*time.Second

	first := jobs.Delay(base, ceiling, 1)
	if first < base || first > base*2 {
		t.Fatalf("the first retry waits %v, want roughly %v", first, base)
	}

	second := jobs.Delay(base, ceiling, 2)
	if second < 2*base {
		t.Fatalf("the second retry waits %v, want at least %v", second, 2*base)
	}

	for attempt := 1; attempt < 40; attempt++ {
		delay := jobs.Delay(base, ceiling, attempt)
		if delay <= 0 {
			t.Fatalf("attempt %d produced a delay of %v", attempt, delay)
		}
		if delay > ceiling+ceiling/4 {
			t.Fatalf("attempt %d waits %v, past the %v ceiling", attempt, delay, ceiling)
		}
	}
}

func TestBackoffIsJitteredSoRetriesDoNotAlign(t *testing.T) {
	seen := map[time.Duration]bool{}
	for range 40 {
		seen[jobs.Delay(time.Second, time.Minute, 3)] = true
	}
	if len(seen) < 2 {
		t.Fatal("40 identical backoff calculations produced one value, so nothing is jittered")
	}
}

func TestBackoffSurvivesAbsurdInput(t *testing.T) {
	if delay := jobs.Delay(0, 0, 0); delay <= 0 {
		t.Fatalf("a zero base and ceiling produced %v", delay)
	}
	if delay := jobs.Delay(-time.Second, -time.Second, -5); delay <= 0 {
		t.Fatalf("negative input produced %v", delay)
	}
	if delay := jobs.Delay(time.Hour, time.Second, 10); delay > 2*time.Second {
		t.Fatalf("a base past the ceiling produced %v, want it capped", delay)
	}
	if delay := jobs.Delay(time.Second, 0, 1000); delay > jobs.MaxBackoff+jobs.MaxBackoff/4 {
		t.Fatalf("1000 attempts with no ceiling produced %v, want it capped at %v", delay, jobs.MaxBackoff)
	}
}
