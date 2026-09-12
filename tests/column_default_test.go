package tests

import (
	"testing"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

type supportTicket struct {
	ID       string `gorm:"primaryKey;size:36"`
	Title    string `gorm:"size:200;not null"`
	Status   string `gorm:"size:20;default:pending"`
	Priority int    `gorm:"default:3"`
	Open     bool   `gorm:"default:true"`
}

type quotedDefault struct {
	ID    string `gorm:"primaryKey;size:36"`
	Label string `gorm:"size:40;default:'already quoted'"`
	Motto string `gorm:"size:40;default:it's fine"`
}

func describedDefaults(t *testing.T, entity any) map[string]string {
	t.Helper()
	a := migratedApp(t, entity)
	schema, err := a.Describe(entity)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, field := range schema.Fields {
		out[field.Column] = field.Default
	}
	return out
}

func TestAStringDefaultBecomesASQLLiteral(t *testing.T) {
	defaults := describedDefaults(t, supportTicket{})
	if defaults["status"] != "'pending'" {
		t.Errorf("status default = %q, want 'pending' quoted as a literal", defaults["status"])
	}
}

func TestANonStringDefaultIsLeftAsWritten(t *testing.T) {
	defaults := describedDefaults(t, supportTicket{})
	if defaults["priority"] != "3" {
		t.Errorf("priority default = %q, want 3 unquoted", defaults["priority"])
	}
	if defaults["open"] != "true" {
		t.Errorf("open default = %q, want true unquoted", defaults["open"])
	}
}

func TestADefaultThatIsAlreadyQuotedIsNotQuotedTwice(t *testing.T) {
	defaults := describedDefaults(t, quotedDefault{})
	if defaults["label"] != "'already quoted'" {
		t.Errorf("label default = %q, want it left alone", defaults["label"])
	}
}

func TestAQuoteInsideADefaultIsEscaped(t *testing.T) {
	defaults := describedDefaults(t, quotedDefault{})
	if defaults["motto"] != "'it''s fine'" {
		t.Errorf("motto default = %q, want the inner quote doubled", defaults["motto"])
	}
}

func TestOnEveryEngineAStringDefaultSurvivesMigrate(t *testing.T) {
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
			a.RegisterModel(model.Of(supportTicket{}))
			t.Cleanup(func() { _ = a.CloseDB() })

			handle, err := a.DB()
			if err != nil {
				t.Fatalf("opening %s: %v", target.name, err)
			}
			for _, table := range []any{&supportTicket{}, migrate.LedgerTable} {
				if !handle.Migrator().HasTable(table) {
					continue
				}
				if err := handle.Migrator().DropTable(table); err != nil {
					t.Fatalf("clearing %v on %s: %v", table, target.name, err)
				}
			}

			change := migrate.Diff(migrate.Snapshot{},
				migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, supportTicket{})}))
			if change.Empty() {
				t.Fatal("creating a missing table should produce ops")
			}

			runner := migrate.NewRunner(handle, cfg.Databases[0].Engine, nil)
			if err := runner.Prepare(t.Context()); err != nil {
				t.Fatalf("preparing the ledger on %s: %v", target.name, err)
			}
			if err := runner.Apply(t.Context(), migrate.Migration{ID: "0001_tickets", Up: change.Ops}); err != nil {
				t.Fatalf("applying the generated migration on %s: %v", target.name, err)
			}

			if err := handle.Table("support_tickets").Create(map[string]any{
				"id": "t1", "title": "Broken kettle",
			}).Error; err != nil {
				t.Fatalf("inserting without a status on %s: %v", target.name, err)
			}

			var stored supportTicket
			if err := handle.First(&stored, "id = ?", "t1").Error; err != nil {
				t.Fatalf("reading back on %s: %v", target.name, err)
			}
			if stored.Status != "pending" {
				t.Errorf("status = %q on %s, want the column default to have applied", stored.Status, target.name)
			}
		})
	}
}
