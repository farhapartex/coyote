package tests

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

type syncBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String()
}

type workerApp struct {
	cfg    settings.Settings
	handle *gorm.DB
	runner *jobs.Runner
	err    error
	mu     sync.Mutex
	asked  askedFor
	closed bool
}

type askedFor struct {
	workers int
	queues  []string
}

func (w *workerApp) Serve() error { return nil }

func (w *workerApp) Config() settings.Settings { return w.cfg }

func (w *workerApp) Models() []model.Model { return nil }

func (w *workerApp) DB() (*gorm.DB, error) { return w.handle, nil }

func (w *workerApp) AuthService() *auth.Service { return nil }

func (w *workerApp) Log() *slog.Logger { return quietLogger() }

func (w *workerApp) Worker(workers int, queues []string) (*jobs.Runner, error) {
	w.mu.Lock()
	w.asked = askedFor{workers: workers, queues: queues}
	w.mu.Unlock()
	return w.runner, w.err
}

func (w *workerApp) requested() askedFor {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.asked
}

func (w *workerApp) CloseDB() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func TestTheWorkerCommandIsOnTheCommandSurface(t *testing.T) {
	names := cli.Default().Names()
	for _, name := range names {
		if name == cli.NameWorker {
			return
		}
	}
	t.Fatalf("worker is not a registered command; the surface is %v", names)
}

func TestTheWorkerCommandRefusesWhenJobsAreOff(t *testing.T) {
	host := &workerApp{cfg: devSettings(t)}
	out := &bytes.Buffer{}

	err := cli.Default().Run(cli.Context{App: host, Out: out}, cli.NameWorker)
	if err == nil {
		t.Fatal("the worker started while jobs are off")
	}
	if !strings.Contains(err.Error(), "Jobs.Enabled is false") {
		t.Fatalf("the error does not say why: %v", err)
	}
}

func TestTheWorkerCommandRefusesWithNoQueue(t *testing.T) {
	host := &workerApp{
		cfg: devSettings(t, func(s *settings.Settings) { s.Jobs.Enabled = true }),
		err: jobs.ErrNoQueue,
	}

	err := cli.Default().Run(cli.Context{App: host, Out: &bytes.Buffer{}}, cli.NameWorker)
	if err == nil {
		t.Fatal("the worker started with no queue")
	}
	if !strings.Contains(err.Error(), "no job queue is configured") {
		t.Fatalf("the error does not say why: %v", err)
	}
}

func TestTheWorkerCommandPassesItsFlagsThroughAndDrainsOnASignal(t *testing.T) {
	queue, _ := newJobQueue(t)
	runner := jobs.NewRunner(jobs.RunnerOptions{
		Queue:        queue,
		Registry:     jobs.NewRegistry(),
		Schedule:     jobs.NewSchedule(),
		Workers:      2,
		PollInterval: 10 * time.Millisecond,
		DrainTimeout: time.Second,
		Logger:       quietLogger(),
	})

	host := &workerApp{
		cfg:    devSettings(t, func(s *settings.Settings) { s.Jobs.Enabled = true }),
		runner: runner,
	}
	command := cli.Worker{Workers: 3, Queues: []string{"email", "reports"}}
	out := &syncBuffer{}

	finished := make(chan error, 1)
	go func() {
		finished <- command.Run(cli.Context{App: host, Out: out})
	}()

	waitFor(t, "the worker to announce its queues", func() bool {
		return strings.Contains(out.String(), "email, reports")
	})
	asked := host.requested()
	if asked.workers != 3 {
		t.Fatalf("the command asked for %d workers, want 3", asked.workers)
	}
	if len(asked.queues) != 2 || asked.queues[0] != "email" {
		t.Fatalf("the command asked for queues %v", asked.queues)
	}

	if err := runner.Close(); err != nil {
		t.Fatalf("closing the runner: %v", err)
	}
}

func TestTheWorkerCommandReadsItsFlagsFromTheEnvironment(t *testing.T) {
	t.Setenv(cli.EnvWorkers, "6")
	t.Setenv(cli.EnvQueues, " email , reports ,, ")

	command := cli.WorkerFromEnv()
	if command.Workers != 6 {
		t.Fatalf("Workers is %d, want 6", command.Workers)
	}
	if len(command.Queues) != 2 || command.Queues[0] != "email" || command.Queues[1] != "reports" {
		t.Fatalf("Queues is %v, want the two names trimmed with the blanks dropped", command.Queues)
	}
}

func TestTheWorkerCommandDefaultsToTheSettings(t *testing.T) {
	command := cli.WorkerFromEnv()
	if command.Workers != 0 || command.Queues != nil {
		t.Fatalf("with no environment set the command is %+v, want zero values so settings win", command)
	}
}

func TestTheAppBuildsAWorkerRunnerOnDemand(t *testing.T) {
	a := newJobApp(t)

	if a.Jobs() != nil {
		t.Fatal("no workers are configured, so there should be no in-process runner")
	}

	runner, err := a.Worker(0, nil)
	if err != nil {
		t.Fatalf("building a worker: %v", err)
	}
	if runner == nil {
		t.Fatal("Worker returned no runner")
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
}

func TestTheAppRefusesAWorkerWhenJobsAreOff(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.Worker(1, nil); err == nil {
		t.Fatal("a worker was built while jobs are off")
	}
}

func TestAWorkerRunnerUsesTheQueuesItWasGiven(t *testing.T) {
	a := newJobApp(t, func(s *settings.Settings) { s.Jobs.Queues = []string{"default", "email"} })

	runner, err := a.Worker(2, []string{"email"})
	if err != nil {
		t.Fatalf("building a worker: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	onEmail, err := jobs.Enqueue(t.Context(), a.Queue(), "only.email", nil, jobs.Options{Queue: "email"})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	onDefault, err := jobs.Enqueue(t.Context(), a.Queue(), "only.default", nil)
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	runner.Start()
	waitFor(t, "the email job to be picked up", func() bool {
		return storedJob(t, handle, onEmail).State != jobs.Queued
	})
	if stored := storedJob(t, handle, onDefault); stored.State != jobs.Queued {
		t.Fatalf("a job on the default queue was taken by a worker restricted to email: %q", stored.State)
	}
}

func TestStartingAndClosingARunnerConcurrentlyIsSafe(t *testing.T) {
	queue, _ := newJobQueue(t)

	for range 20 {
		runner := jobs.NewRunner(jobs.RunnerOptions{
			Queue:        queue,
			Registry:     jobs.NewRegistry(),
			Schedule:     jobs.NewSchedule(),
			Workers:      4,
			PollInterval: time.Millisecond,
			DrainTimeout: time.Second,
			Logger:       quietLogger(),
		})

		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			runner.Start()
		}()
		go func() {
			defer wait.Done()
			_ = runner.Close()
		}()
		wait.Wait()
	}
}

func TestStartingAfterCloseDoesNothing(t *testing.T) {
	queue, _ := newJobQueue(t)
	runner := jobs.NewRunner(jobs.RunnerOptions{
		Queue:    queue,
		Registry: jobs.NewRegistry(),
		Schedule: jobs.NewSchedule(),
		Workers:  2,
		Logger:   quietLogger(),
	})

	if err := runner.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	runner.Start()
	if err := runner.Close(); err != nil {
		t.Fatalf("closing again: %v", err)
	}
}
