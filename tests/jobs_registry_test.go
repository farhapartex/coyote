package tests

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
)

type welcomeArgs struct {
	UserID string `json:"user_id"`
	Tries  int    `json:"tries"`
}

func TestRegistryDispatchesToTheRegisteredHandler(t *testing.T) {
	registry := jobs.NewRegistry()
	var seen welcomeArgs

	if err := jobs.HandleOn(registry, "email.welcome", func(_ context.Context, args welcomeArgs) error {
		seen = args
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	if err := registry.Run(context.Background(), "email.welcome", []byte(`{"user_id":"u1","tries":3}`)); err != nil {
		t.Fatalf("running: %v", err)
	}
	if seen.UserID != "u1" || seen.Tries != 3 {
		t.Fatalf("handler saw %+v", seen)
	}
}

func TestRegistryRefusesADuplicateKind(t *testing.T) {
	registry := jobs.NewRegistry()
	run := func(_ context.Context, _ welcomeArgs) error { return nil }

	if err := jobs.HandleOn(registry, "email.welcome", run); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	err := jobs.HandleOn(registry, "email.welcome", run)
	if !errors.Is(err, jobs.ErrDuplicateKind) {
		t.Fatalf("second registration returned %v, want ErrDuplicateKind", err)
	}
}

func TestRegistryRefusesAnEmptyKindAndANilFunction(t *testing.T) {
	registry := jobs.NewRegistry()

	if err := jobs.HandleOn(registry, "", func(_ context.Context, _ welcomeArgs) error { return nil }); !errors.Is(err, jobs.ErrKindMissing) {
		t.Fatalf("empty kind returned %v, want ErrKindMissing", err)
	}
	if err := jobs.HandleOn[welcomeArgs](registry, "email.welcome", nil); err == nil {
		t.Fatal("a nil function was accepted")
	}
	if registry.Knows("email.welcome") {
		t.Fatal("a rejected registration still landed in the registry")
	}
}

func TestRunningAnUnregisteredKindIsRefused(t *testing.T) {
	registry := jobs.NewRegistry()
	err := registry.Run(context.Background(), "email.welcome", nil)
	if !errors.Is(err, jobs.ErrUnknownKind) {
		t.Fatalf("got %v, want ErrUnknownKind", err)
	}
}

func TestAPayloadThatDoesNotDecodeIsRefusedBeforeTheHandlerRuns(t *testing.T) {
	registry := jobs.NewRegistry()
	ran := false

	if err := jobs.HandleOn(registry, "email.welcome", func(_ context.Context, _ welcomeArgs) error {
		ran = true
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	err := registry.Run(context.Background(), "email.welcome", []byte(`{"user_id":[1,2,3]}`))
	if !errors.Is(err, jobs.ErrPayloadInvalid) {
		t.Fatalf("got %v, want ErrPayloadInvalid", err)
	}
	if ran {
		t.Fatal("the handler ran on a payload that did not decode")
	}
}

func TestAnEmptyPayloadReachesTheHandlerAsItsZeroValue(t *testing.T) {
	registry := jobs.NewRegistry()
	var seen welcomeArgs
	called := false

	if err := jobs.HandleOn(registry, "cache.warm", func(_ context.Context, args welcomeArgs) error {
		seen, called = args, true
		return nil
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	if err := registry.Run(context.Background(), "cache.warm", nil); err != nil {
		t.Fatalf("running: %v", err)
	}
	if !called || seen != (welcomeArgs{}) {
		t.Fatalf("called=%v saw %+v", called, seen)
	}
}

func TestTheHandlersErrorReachesTheCaller(t *testing.T) {
	registry := jobs.NewRegistry()
	sentinel := errors.New("smtp refused the message")

	if err := jobs.HandleOn(registry, "email.welcome", func(_ context.Context, _ welcomeArgs) error {
		return sentinel
	}); err != nil {
		t.Fatalf("registering: %v", err)
	}

	if err := registry.Run(context.Background(), "email.welcome", nil); !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want the handler's own error", err)
	}
}

func TestKindsAreListedInOrder(t *testing.T) {
	registry := jobs.NewRegistry()
	for _, kind := range []string{"report.build", "email.welcome", "cache.warm"} {
		if err := jobs.HandleOn(registry, kind, func(_ context.Context, _ welcomeArgs) error { return nil }); err != nil {
			t.Fatalf("registering %s: %v", kind, err)
		}
	}

	want := []string{"cache.warm", "email.welcome", "report.build"}
	got := registry.Kinds()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestTheRegistryIsSafeUnderConcurrentRegistrationAndLookup(t *testing.T) {
	registry := jobs.NewRegistry()
	var wait sync.WaitGroup

	for i := range 50 {
		wait.Add(2)
		kind := "job." + strconv.Itoa(i)
		go func() {
			defer wait.Done()
			_ = jobs.HandleOn(registry, kind, func(_ context.Context, _ welcomeArgs) error { return nil })
		}()
		go func() {
			defer wait.Done()
			registry.Knows(kind)
			registry.Kinds()
		}()
	}
	wait.Wait()

	if len(registry.Kinds()) != 50 {
		t.Fatalf("registered %d kinds, want 50", len(registry.Kinds()))
	}
}

func TestHandlePanicsOnADuplicateSoItIsCaughtAtStartup(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a duplicate registration on the default registry did not panic")
		}
		err, ok := recovered.(error)
		if !ok || !errors.Is(err, jobs.ErrDuplicateKind) {
			t.Fatalf("panicked with %v, want ErrDuplicateKind", recovered)
		}
	}()

	run := func(_ context.Context, _ welcomeArgs) error { return nil }
	jobs.Handle("tests.duplicate.probe", run)
	jobs.Handle("tests.duplicate.probe", run)
}

func TestEnqueueWithoutAQueueIsRefused(t *testing.T) {
	_, err := jobs.Enqueue(context.Background(), nil, "email.welcome", welcomeArgs{})
	if !errors.Is(err, jobs.ErrNoQueue) {
		t.Fatalf("got %v, want ErrNoQueue", err)
	}
}

func TestEnqueueAppliesTheDefaultQueueAndTheRequestedDelay(t *testing.T) {
	recorder := &recordingQueue{}
	before := time.Now()

	if _, err := jobs.Enqueue(context.Background(), recorder, "email.welcome", welcomeArgs{UserID: "u1"}); err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if recorder.last.Queue != jobs.DefaultQueue {
		t.Fatalf("queue is %q, want %q", recorder.last.Queue, jobs.DefaultQueue)
	}
	if recorder.last.RunAt.Before(before) || recorder.last.RunAt.After(time.Now()) {
		t.Fatalf("run_at %v is outside the enqueue window", recorder.last.RunAt)
	}
	if string(recorder.last.Payload) != `{"user_id":"u1","tries":0}` {
		t.Fatalf("payload is %s", recorder.last.Payload)
	}

	if _, err := jobs.Enqueue(context.Background(), recorder, "email.welcome", welcomeArgs{}, jobs.Options{
		Queue: "email", Delay: time.Hour, Priority: 5, MaxAttempts: 9, Fingerprint: "once",
	}); err != nil {
		t.Fatalf("enqueueing with options: %v", err)
	}
	if recorder.last.Queue != "email" || recorder.last.Priority != 5 || recorder.last.MaxAttempts != 9 {
		t.Fatalf("options were not carried: %+v", recorder.last)
	}
	if recorder.last.Fingerprint != "once" {
		t.Fatalf("fingerprint is %q", recorder.last.Fingerprint)
	}
	if time.Until(recorder.last.RunAt) < 59*time.Minute {
		t.Fatalf("delay was not applied: run_at is %v", recorder.last.RunAt)
	}
}

func TestEnqueueRefusesAJobWithNoKind(t *testing.T) {
	recorder := &recordingQueue{}
	if _, err := jobs.Enqueue(context.Background(), recorder, "", welcomeArgs{}); !errors.Is(err, jobs.ErrKindMissing) {
		t.Fatalf("got %v, want ErrKindMissing", err)
	}
	if recorder.calls != 0 {
		t.Fatal("a job with no kind reached the queue")
	}
}

func TestStatesAndRecordPredicates(t *testing.T) {
	if len(jobs.States()) != 4 {
		t.Fatalf("States() returned %v", jobs.States())
	}
	if jobs.Queued.Terminal() || jobs.Running.Terminal() {
		t.Fatal("queued and running are not terminal states")
	}
	if !jobs.Done.Terminal() || !jobs.Dead.Terminal() {
		t.Fatal("done and dead are terminal states")
	}

	record := jobs.Record{Attempts: 2, MaxAttempts: 3, RunAt: time.Now().Add(-time.Minute)}
	if record.Exhausted() {
		t.Fatal("2 of 3 attempts is not exhausted")
	}
	record.Attempts = 3
	if !record.Exhausted() {
		t.Fatal("3 of 3 attempts is exhausted")
	}
	if !record.Due(time.Now()) {
		t.Fatal("a job whose run_at has passed is due")
	}
	later := jobs.Record{RunAt: time.Now().Add(time.Hour)}
	if later.Due(time.Now()) {
		t.Fatal("a job scheduled for later is not due")
	}
}

func TestOnlyForeverMeansNoAttemptCeiling(t *testing.T) {
	unlimited := jobs.Record{Attempts: 500, MaxAttempts: jobs.Forever}
	if unlimited.Exhausted() {
		t.Fatal("MaxAttempts of Forever means no ceiling, so the job is never exhausted")
	}

	unset := jobs.Record{Attempts: 1, MaxAttempts: 0}
	if !unset.Exhausted() {
		t.Fatal("a row with no ceiling written to it must fail closed, not retry forever")
	}
}

func TestTheJobRecordDescribesAsASchemaMakemigrationsCanUse(t *testing.T) {
	a := newTestApp(t)

	schema, err := a.Describe(jobs.Record{})
	if err != nil {
		t.Fatalf("describing: %v", err)
	}
	if schema.Table != "jobs" {
		t.Fatalf("table is %q, want jobs", schema.Table)
	}
	if schema.Key.Column != "id" {
		t.Fatalf("primary key is %q, want id", schema.Key.Column)
	}

	for _, column := range []string{
		"queue", "kind", "state", "run_at", "priority", "payload",
		"attempts", "max_attempts", "fingerprint", "locked_by", "locked_at",
		"last_error", "finished_at", "created_at", "updated_at",
	} {
		if !schema.HasColumn(column) {
			t.Fatalf("the schema has no %q column", column)
		}
	}

	fingerprint, _ := schema.Field("fingerprint")
	if fingerprint.Sensitive {
		t.Fatal("fingerprint was flagged sensitive, so the admin would render it as a password")
	}
	if !fingerprint.Nullable {
		t.Fatal("fingerprint must be nullable, or a second job with no fingerprint collides on the unique index")
	}

	lastError, _ := schema.Field("last_error")
	if lastError.Kind != model.KindText {
		t.Fatalf("last_error is %q, want text so the admin renders a textarea", lastError.Kind)
	}

	payload, _ := schema.Field("payload")
	if payload.Kind != model.KindBytes {
		t.Fatalf("payload is %q, want bytes", payload.Kind)
	}

	state, _ := schema.Field("state")
	if state.Kind != model.KindString {
		t.Fatalf("state is %q, want string", state.Kind)
	}
}

func TestTheJobRecordDeclaresTheIndexTheClaimLoopNeeds(t *testing.T) {
	a := newTestApp(t)

	schema, err := a.Describe(jobs.Record{})
	if err != nil {
		t.Fatalf("describing: %v", err)
	}

	var claim []string
	unique := map[string]bool{}
	for _, index := range schema.Indexes {
		if index.Name == "idx_jobs_claim" {
			claim = index.Columns
		}
		if index.Unique {
			for _, column := range index.Columns {
				unique[column] = true
			}
		}
	}

	want := []string{"queue", "state", "run_at", "priority"}
	if len(claim) != len(want) {
		t.Fatalf("idx_jobs_claim covers %v, want %v", claim, want)
	}
	for i := range want {
		if claim[i] != want[i] {
			t.Fatalf("idx_jobs_claim covers %v, want %v", claim, want)
		}
	}
	if !unique["fingerprint"] {
		t.Fatal("fingerprint needs a unique index, or the scheduler cannot deduplicate")
	}
}

type recordingQueue struct {
	last  jobs.Job
	calls int
}

func (q *recordingQueue) Enqueue(_ context.Context, job jobs.Job) (string, error) {
	q.last = job
	q.calls++
	return "job-1", nil
}

func (q *recordingQueue) Claim(context.Context, string, []string, int) ([]jobs.Record, error) {
	return nil, nil
}

func (q *recordingQueue) Complete(context.Context, string, time.Time) error { return nil }

func (q *recordingQueue) Fail(context.Context, string, string, *time.Time) error { return nil }

func (q *recordingQueue) Retry(context.Context, string, time.Time) error { return nil }

func (q *recordingQueue) Heartbeat(context.Context, string, time.Time) error { return nil }

func (q *recordingQueue) Recover(context.Context, time.Time) (int, error) { return 0, nil }

func (q *recordingQueue) Sweep(context.Context, time.Time) (int, error) { return 0, nil }

func (q *recordingQueue) Stats(context.Context) (jobs.Stats, error) { return jobs.Stats{}, nil }

func (q *recordingQueue) WithTx(*gorm.DB) jobs.Queue { return q }
