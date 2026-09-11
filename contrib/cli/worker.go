package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/farhapartex/coyote/core/jobs"
)

type JobWorker interface {
	Worker(workers int, queues []string) (*jobs.Runner, error)
}

type Worker struct {
	Workers int
	Queues  []string
}

func (Worker) Name() string { return NameWorker }

func (Worker) Summary() string { return "run background jobs as their own process" }

func WorkerFromEnv() Worker {
	return Worker{
		Workers: flagInt(EnvWorkers, 0),
		Queues:  flagList(EnvQueues),
	}
}

func (c Worker) Run(ctx Context) error {
	host, ok := ctx.App.(JobWorker)
	if !ok {
		return errors.New("coyote/cli: this application cannot run background jobs")
	}
	if !ctx.App.Config().Jobs.Enabled {
		return errors.New("coyote/cli: Jobs.Enabled is false in settings.go, so there is no queue to work")
	}

	runner, err := host.Worker(c.Workers, c.Queues)
	if err != nil {
		if errors.Is(err, jobs.ErrNoQueue) {
			return errors.New("coyote/cli: no job queue is configured; set a database or supply Jobs.Queue")
		}
		return err
	}

	queues := c.Queues
	if len(queues) == 0 {
		queues = ctx.App.Config().Jobs.QueueNames()
	}
	fmt.Fprintf(ctx.Out, "coyote worker on %s\n", strings.Join(queues, ", "))

	runner.Start()

	signalled, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-signalled.Done()

	fmt.Fprintln(ctx.Out, "draining jobs")
	if err := runner.Close(); err != nil {
		return err
	}
	if closer, ok := ctx.App.(interface{ CloseDB() error }); ok {
		return closer.CloseDB()
	}
	return nil
}
