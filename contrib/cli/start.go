package cli

import (
	"context"
	"log/slog"

	"github.com/farhapartex/coyote/contrib/migrate"
)

type Start struct{}

func (Start) Name() string { return NameStart }

func (Start) Summary() string { return "build and run the project in the current directory" }

func (Start) Run(ctx Context) error {
	ReportMigrationState(ctx.App)
	return ctx.App.Serve()
}

func ReportMigrationState(app Application) {
	log := app.Log()
	database := app.Config().Database()
	if database.Engine == "" {
		return
	}

	handle, err := app.DB()
	if err != nil || handle == nil {
		log.Warn("cannot check migrations, the database is unreachable",
			slog.String("database", database.Name),
			slog.Any("error", err))
		return
	}

	runner := migrate.NewRunner(handle, database.Engine, migrate.Registered())
	state, err := runner.State(context.Background())
	if err != nil {
		log.Warn("cannot check migrations", slog.Any("error", err))
		return
	}

	switch {
	case state.Declared == 0 && state.FreshDatabase():
		log.Warn("this database has no schema yet and no migrations are declared; " +
			"run: go tool coyote makemigrations && go tool coyote migrate")
	case state.FreshDatabase():
		log.Warn("no migrations have been applied to this database; nothing will work until you run: go tool coyote migrate",
			slog.Int("pending", len(state.Pending)))
	case len(state.Pending) > 0:
		log.Warn("migrations are pending; run: go tool coyote migrate",
			slog.Int("pending", len(state.Pending)),
			slog.Int("applied", state.Applied))
	default:
		log.Info("migrations up to date", slog.Int("applied", state.Applied))
	}

	if !state.FreshDatabase() && app.AuthService().Users().Count() == 0 {
		log.Warn("there are no users yet, so nobody can sign in; run: go tool coyote createsuperadmin")
	}
}
