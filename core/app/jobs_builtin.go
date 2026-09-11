package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/farhapartex/coyote/core/jobs"
)

const (
	SweepUploads = "coyote.uploads.sweep"
	SweepTokens  = "coyote.tokens.sweep"

	sweepUploadsEvery = 6 * time.Hour
	sweepTokensEvery  = 24 * time.Hour
)

func (a *App) builtinHandlers() *jobs.Registry {
	own := jobs.RegistryUnder(jobs.Handlers())

	if a.Uploads != nil {
		a.register(own, SweepUploads, func(ctx context.Context) error {
			staged, trashed, err := a.Uploads.Sweep(ctx)
			if err != nil {
				return err
			}
			if staged+trashed > 0 {
				a.Logger.Info("uploads swept",
					slog.Int("staged", staged), slog.Int("trashed", trashed))
			}
			return nil
		})
	}
	if tokens := a.Auth.Tokens(); tokens != nil {
		a.register(own, SweepTokens, func(ctx context.Context) error {
			removed, err := tokens.Sweep(ctx, time.Now())
			if err != nil {
				return err
			}
			if removed > 0 {
				a.Logger.Info("expired reset tokens swept", slog.Int("tokens", removed))
			}
			return nil
		})
	}
	return own
}

func (a *App) register(target *jobs.Registry, kind string, run func(context.Context) error) {
	err := jobs.HandleOn(target, kind, func(ctx context.Context, _ struct{}) error {
		return run(ctx)
	})
	if err != nil {
		a.Logger.Error("a built-in job could not be registered",
			slog.String("kind", kind), slog.Any("error", err))
	}
}

func (a *App) builtinSchedule(handlers *jobs.Registry) *jobs.Schedule {
	own := jobs.ScheduleUnder(jobs.Schedules())

	for kind, every := range map[string]time.Duration{
		SweepUploads: sweepUploadsEvery,
		SweepTokens:  sweepTokensEvery,
	} {
		if !handlers.Knows(kind) || jobs.Schedules().Holds(kind) {
			continue
		}
		if err := own.Add(jobs.Periodic{Kind: kind, Every: every}); err != nil {
			a.Logger.Error("a built-in job could not be scheduled",
				slog.String("kind", kind), slog.Any("error", err))
		}
	}
	return own
}
