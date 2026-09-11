package app

import (
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/store"
)

func jobQueue(s Settings, a *App) jobs.Queue {
	if !s.Jobs.Enabled {
		return nil
	}
	if s.Jobs.Queue != nil {
		return s.Jobs.Queue
	}
	if s.Database().Engine == "" {
		return nil
	}
	return store.LazyJobs(a.DB, store.JobOptions{MaxAttempts: s.Jobs.MaxAttempts})
}

func (a *App) Queue() jobs.Queue { return a.queue }
