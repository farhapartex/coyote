package tests

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
	"gorm.io/gorm"
)

func newJobQueue(t *testing.T) (jobs.Queue, *gorm.DB) {
	t.Helper()
	a := app.NewFrom(devSettings(t))
	a.RegisterModel(model.Of(jobs.Record{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = a.CloseDB() })
	return store.Jobs(handle, store.JobOptions{}), handle
}

func enqueued(t *testing.T, queue jobs.Queue, kind string, opts jobs.Options) string {
	t.Helper()
	id, err := jobs.Enqueue(context.Background(), queue, kind, nil, opts)
	if err != nil {
		t.Fatalf("enqueueing %s: %v", kind, err)
	}
	return id
}

func storedJob(t *testing.T, handle *gorm.DB, id string) jobs.Record {
	t.Helper()
	var record jobs.Record
	if err := handle.Where("id = ?", id).First(&record).Error; err != nil {
		t.Fatalf("reading job %s: %v", id, err)
	}
	return record
}

func TestAClaimedJobIsRunningAndCountsAnAttempt(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "email.welcome", jobs.Options{})

	claimed, err := queue.Claim(context.Background(), "worker-1", nil, 5)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != id {
		t.Fatalf("claimed %d jobs: %+v", len(claimed), claimed)
	}
	if claimed[0].State != jobs.Running || claimed[0].Attempts != 1 || claimed[0].LockedBy != "worker-1" {
		t.Fatalf("the returned record is not a running claim: %+v", claimed[0])
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Running || stored.Attempts != 1 || stored.LockedBy != "worker-1" {
		t.Fatalf("the stored row does not match the claim: %+v", stored)
	}
	if stored.LockedAt == nil {
		t.Fatal("a claimed job has no locked_at, so Recover could never reclaim it")
	}
}

func TestAJobScheduledForLaterIsNotClaimed(t *testing.T) {
	queue, _ := newJobQueue(t)
	enqueued(t, queue, "email.welcome", jobs.Options{Delay: time.Hour})

	claimed, err := queue.Claim(context.Background(), "worker-1", nil, 5)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("claimed a job whose run_at has not arrived: %+v", claimed)
	}
}

func TestClaimTakesOnlyTheQueuesItWasAskedFor(t *testing.T) {
	queue, _ := newJobQueue(t)
	enqueued(t, queue, "email.welcome", jobs.Options{Queue: "email"})
	enqueued(t, queue, "report.build", jobs.Options{Queue: "reports"})

	claimed, err := queue.Claim(context.Background(), "worker-1", []string{"email"}, 5)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Queue != "email" {
		t.Fatalf("claimed %+v, want only the email queue", claimed)
	}
}

func TestClaimPrefersHigherPriorityThenOlderJobs(t *testing.T) {
	queue, _ := newJobQueue(t)
	low := enqueued(t, queue, "low", jobs.Options{Priority: 1})
	high := enqueued(t, queue, "high", jobs.Options{Priority: 9})
	middle := enqueued(t, queue, "middle", jobs.Options{Priority: 5})

	claimed, err := queue.Claim(context.Background(), "worker-1", nil, 3)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if len(claimed) != 3 {
		t.Fatalf("claimed %d jobs, want 3", len(claimed))
	}
	if claimed[0].ID != high || claimed[1].ID != middle || claimed[2].ID != low {
		t.Fatalf("claim order was %s %s %s, want high middle low",
			claimed[0].Kind, claimed[1].Kind, claimed[2].Kind)
	}
}

func TestClaimNeverReturnsMoreThanTheLimit(t *testing.T) {
	queue, _ := newJobQueue(t)
	for i := range 10 {
		enqueued(t, queue, "bulk."+strconv.Itoa(i), jobs.Options{})
	}

	claimed, err := queue.Claim(context.Background(), "worker-1", nil, 4)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if len(claimed) != 4 {
		t.Fatalf("claimed %d jobs, want 4", len(claimed))
	}
}

func TestOneJobIsClaimedByExactlyOneOfManyCompetingWorkers(t *testing.T) {
	queue, _ := newJobQueue(t)
	for i := range 3 {
		enqueued(t, queue, "contended."+strconv.Itoa(i), jobs.Options{})
	}

	var (
		mutex   sync.Mutex
		claimed []string
		wait    sync.WaitGroup
	)
	for worker := range 8 {
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

	if len(claimed) != 3 {
		t.Fatalf("8 workers claimed %d jobs in total, want exactly the 3 that existed", len(claimed))
	}
	seen := map[string]bool{}
	for _, id := range claimed {
		if seen[id] {
			t.Fatalf("job %s was claimed twice", id)
		}
		seen[id] = true
	}
}

func TestCompletingAJobFinishesItAndOnlyWorksWhileItRuns(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "email.welcome", jobs.Options{})

	if err := queue.Complete(context.Background(), id, time.Now()); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("completing a queued job returned %v, want ErrNotFound", err)
	}

	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if err := queue.Complete(context.Background(), id, time.Now()); err != nil {
		t.Fatalf("completing: %v", err)
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Done || stored.FinishedAt == nil || stored.LockedBy != "" {
		t.Fatalf("a completed job looks like %+v", stored)
	}
	if stored.LockedAt != nil {
		t.Fatal("a completed job still holds its lock")
	}
}

func TestFailingWithARetryPutsTheJobBackAtItsNewRunAt(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "email.welcome", jobs.Options{})
	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	retryAt := time.Now().Add(30 * time.Minute)
	if err := queue.Fail(context.Background(), id, "smtp timed out", &retryAt); err != nil {
		t.Fatalf("failing: %v", err)
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Queued || stored.LastError != "smtp timed out" {
		t.Fatalf("a retried job looks like %+v", stored)
	}
	if stored.LockedBy != "" || stored.LockedAt != nil {
		t.Fatal("a retried job still holds its lock")
	}
	if stored.Attempts != 1 {
		t.Fatalf("attempts is %d, want the one the claim counted", stored.Attempts)
	}

	claimed, err := queue.Claim(context.Background(), "worker-2", nil, 1)
	if err != nil {
		t.Fatalf("claiming after the retry: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatal("a job waiting on its backoff was claimed early")
	}
}

func TestFailingWithNoRetryBuriesTheJob(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "email.welcome", jobs.Options{})
	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	if err := queue.Fail(context.Background(), id, "the address does not exist", nil); err != nil {
		t.Fatalf("failing: %v", err)
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Dead || stored.FinishedAt == nil {
		t.Fatalf("a buried job looks like %+v", stored)
	}
	if stored.LastError != "the address does not exist" {
		t.Fatalf("last_error is %q", stored.LastError)
	}
}

func TestAHeartbeatPushesTheLockForwardAndNeedsARunningJob(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "report.build", jobs.Options{})

	if err := queue.Heartbeat(context.Background(), id, time.Now()); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("a heartbeat on a queued job returned %v, want ErrNotFound", err)
	}

	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	first := storedJob(t, handle, id)

	later := time.Now().Add(time.Minute)
	if err := queue.Heartbeat(context.Background(), id, later); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	beating := storedJob(t, handle, id)
	if !beating.LockedAt.After(*first.LockedAt) {
		t.Fatalf("locked_at did not move: %v then %v", first.LockedAt, beating.LockedAt)
	}
}

func TestRecoverReclaimsAnAbandonedJobAndLeavesALiveOneAlone(t *testing.T) {
	queue, handle := newJobQueue(t)
	abandoned := enqueued(t, queue, "report.build", jobs.Options{})
	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	recovered, err := queue.Recover(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("recovering: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("recovered %d jobs, want none while the lock is fresh", recovered)
	}

	recovered, err = queue.Recover(context.Background(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("recovering: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered %d jobs, want 1", recovered)
	}

	stored := storedJob(t, handle, abandoned)
	if stored.State != jobs.Queued || stored.LockedBy != "" || stored.LockedAt != nil {
		t.Fatalf("a recovered job looks like %+v", stored)
	}
	if stored.Attempts != 1 {
		t.Fatalf("attempts is %d, want the attempt the dead worker used", stored.Attempts)
	}
}

func TestRecoverBuriesAnAbandonedJobThatHasNoAttemptsLeft(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "report.build", jobs.Options{MaxAttempts: 1})
	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	recovered, err := queue.Recover(context.Background(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("recovering: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered %d jobs, want 1", recovered)
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Dead {
		t.Fatalf("an abandoned job with no attempts left is %q, want dead", stored.State)
	}
	if stored.FinishedAt == nil || stored.LastError == "" {
		t.Fatalf("a buried job carries no reason: %+v", stored)
	}
}

func TestRecoverNeverBuriesAJobThatRetriesForever(t *testing.T) {
	queue, handle := newJobQueue(t)
	id := enqueued(t, queue, "report.build", jobs.Options{MaxAttempts: jobs.Forever})
	if _, err := queue.Claim(context.Background(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	if _, err := queue.Recover(context.Background(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("recovering: %v", err)
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Queued {
		t.Fatalf("a job that retries forever became %q", stored.State)
	}
}

func TestSweepRemovesFinishedJobsAndKeepsTheRest(t *testing.T) {
	queue, handle := newJobQueue(t)
	done := enqueued(t, queue, "done.one", jobs.Options{})
	dead := enqueued(t, queue, "dead.one", jobs.Options{})
	waiting := enqueued(t, queue, "waiting.one", jobs.Options{})

	if _, err := queue.Claim(context.Background(), "worker-1", nil, 2); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if err := queue.Complete(context.Background(), done, time.Now()); err != nil {
		t.Fatalf("completing: %v", err)
	}
	if err := queue.Fail(context.Background(), dead, "gave up", nil); err != nil {
		t.Fatalf("failing: %v", err)
	}

	removed, err := queue.Sweep(context.Background(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("sweeping: %v", err)
	}
	if removed != 1 {
		t.Fatalf("swept %d jobs, want only the finished one", removed)
	}

	var remaining int64
	if err := handle.Model(&jobs.Record{}).Count(&remaining).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("%d jobs remain, want the dead one and the waiting one", remaining)
	}
	storedJob(t, handle, dead)
	storedJob(t, handle, waiting)
}

func TestStatsCountsEveryState(t *testing.T) {
	queue, _ := newJobQueue(t)
	done := enqueued(t, queue, "done.one", jobs.Options{})
	dead := enqueued(t, queue, "dead.one", jobs.Options{})
	enqueued(t, queue, "waiting.one", jobs.Options{})
	enqueued(t, queue, "waiting.two", jobs.Options{})

	if _, err := queue.Claim(context.Background(), "worker-1", nil, 2); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if err := queue.Complete(context.Background(), done, time.Now()); err != nil {
		t.Fatalf("completing: %v", err)
	}
	if err := queue.Fail(context.Background(), dead, "gave up", nil); err != nil {
		t.Fatalf("failing: %v", err)
	}

	stats, err := queue.Stats(context.Background())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Queued != 2 || stats.Done != 1 || stats.Dead != 1 || stats.Running != 0 {
		t.Fatalf("stats are %+v", stats)
	}
	if stats.Total() != 4 {
		t.Fatalf("total is %d, want 4", stats.Total())
	}
	if !stats.Backlogged() {
		t.Fatal("two queued jobs is a backlog")
	}
}

func TestASecondJobWithTheSameFingerprintIsRefused(t *testing.T) {
	queue, _ := newJobQueue(t)
	enqueued(t, queue, "uploads.sweep", jobs.Options{Fingerprint: "uploads.sweep:2026-09-11T14"})

	_, err := jobs.Enqueue(context.Background(), queue, "uploads.sweep", nil,
		jobs.Options{Fingerprint: "uploads.sweep:2026-09-11T14"})
	if !errors.Is(err, jobs.ErrDuplicateFingerprint) {
		t.Fatalf("got %v, want ErrDuplicateFingerprint", err)
	}
}

func TestManyJobsWithNoFingerprintDoNotCollide(t *testing.T) {
	queue, handle := newJobQueue(t)
	for i := range 5 {
		enqueued(t, queue, "email."+strconv.Itoa(i), jobs.Options{})
	}

	var total int64
	if err := handle.Model(&jobs.Record{}).Count(&total).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if total != 5 {
		t.Fatalf("%d jobs were stored, want 5; an unset fingerprint must be NULL, not an empty string", total)
	}
}

func TestTheAttemptCeilingComesFromTheOptionsOrTheDefault(t *testing.T) {
	queue, handle := newJobQueue(t)

	fallback := enqueued(t, queue, "default.attempts", jobs.Options{})
	if stored := storedJob(t, handle, fallback); stored.MaxAttempts != jobs.DefaultMaxAttempts {
		t.Fatalf("max_attempts is %d, want the default of %d", stored.MaxAttempts, jobs.DefaultMaxAttempts)
	}

	asked := enqueued(t, queue, "explicit.attempts", jobs.Options{MaxAttempts: 7})
	if stored := storedJob(t, handle, asked); stored.MaxAttempts != 7 {
		t.Fatalf("max_attempts is %d, want 7", stored.MaxAttempts)
	}

	endless := enqueued(t, queue, "endless.attempts", jobs.Options{MaxAttempts: jobs.Forever})
	if stored := storedJob(t, handle, endless); stored.MaxAttempts != jobs.Forever {
		t.Fatalf("max_attempts is %d, want Forever", stored.MaxAttempts)
	}
}

func TestAJobEnqueuedInARolledBackTransactionIsNeverStored(t *testing.T) {
	queue, handle := newJobQueue(t)
	refused := errors.New("the caller changed its mind")

	err := handle.Transaction(func(tx *gorm.DB) error {
		if _, err := jobs.Enqueue(context.Background(), queue.WithTx(tx), "email.welcome", nil); err != nil {
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
}

func TestAJobEnqueuedInACommittedTransactionIsStored(t *testing.T) {
	queue, handle := newJobQueue(t)

	err := handle.Transaction(func(tx *gorm.DB) error {
		_, err := jobs.Enqueue(context.Background(), queue.WithTx(tx), "email.welcome", nil)
		return err
	})
	if err != nil {
		t.Fatalf("the transaction failed: %v", err)
	}

	var total int64
	if err := handle.Model(&jobs.Record{}).Count(&total).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if total != 1 {
		t.Fatalf("%d jobs were stored, want 1", total)
	}
}
