package tests

import (
	"testing"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/settings"
)

func TestOnEveryEngineTheMigrationLedgerCanBeCreated(t *testing.T) {
	for _, target := range engines(t) {
		t.Run(target.name, func(t *testing.T) {
			if target.skip != "" {
				t.Skip(target.skip)
			}

			var fns []func(*settings.Settings)
			if target.apply != nil {
				fns = append(fns, target.apply)
			}
			cfg := devSettings(t, fns...)
			a := app.NewFrom(cfg)
			t.Cleanup(func() { _ = a.CloseDB() })

			handle, err := a.DB()
			if err != nil {
				t.Fatalf("opening %s: %v", target.name, err)
			}
			if handle.Migrator().HasTable(migrate.LedgerTable) {
				if err := handle.Migrator().DropTable(migrate.LedgerTable); err != nil {
					t.Fatalf("clearing the ledger on %s: %v", target.name, err)
				}
			}

			runner := migrate.NewRunner(handle, cfg.Databases[0].Engine, nil)
			if err := runner.Prepare(t.Context()); err != nil {
				t.Fatalf("the ledger cannot be created on %s: %v", target.name, err)
			}
			if err := runner.Apply(t.Context(), migrate.Migration{
				ID: "0001_noop",
				Up: []migrate.Op{migrate.RunSQL{Any: "SELECT 1"}},
			}); err != nil {
				t.Fatalf("applying on %s: %v", target.name, err)
			}

			applied, err := runner.Applied(t.Context())
			if err != nil {
				t.Fatalf("reading the ledger on %s: %v", target.name, err)
			}
			if len(applied) != 1 || applied[0].ID != "0001_noop" {
				t.Errorf("ledger on %s = %+v, want one row for 0001_noop", target.name, applied)
			}
		})
	}
}
