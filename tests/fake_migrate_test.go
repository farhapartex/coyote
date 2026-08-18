package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
)

func fakeMigrateApp(t *testing.T) (*fakeApp, *gorm.DB, []migrate.Migration) {
	t.Helper()
	handle := newTestDB(t)
	cfg := devSettings(t)

	pool := []migrate.Migration{
		{
			ID: "0001_initial",
			Up: []migrate.Op{migrate.CreateTable{Table: migrate.Table{
				Name: "widgets",
				Columns: []migrate.Column{
					{Name: "id", Kind: model.KindString, Size: 64, NotNull: true, PrimaryKey: true},
					{Name: "label", Kind: model.KindString, Size: 120},
				},
			}}},
		},
		{
			ID: "0002_add_stock",
			Up: []migrate.Op{migrate.AddColumn{
				Table:  "widgets",
				Column: migrate.Column{Name: "stock", Kind: model.KindInt},
			}},
		},
	}
	return &fakeApp{cfg: cfg, models: []model.Model{}, handle: handle}, handle, pool
}

func runMigrate(t *testing.T, app *fakeApp, command cli.Migrate) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	err := command.Run(cli.Context{App: app, Out: out, In: strings.NewReader("")})
	return out.String(), err
}

func TestFakeRecordsWithoutTouchingTheDatabase(t *testing.T) {
	app, handle, pool := fakeMigrateApp(t)
	if err := migrate.Default().Add(pool...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { migrate.Reset() })

	body, err := runMigrate(t, app, cli.Migrate{Fake: true})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(body, "WITHOUT running anything") {
		t.Errorf("faking should be loud about what it did:\n%s", body)
	}
	for _, id := range []string{"0001_initial", "0002_add_stock"} {
		if !strings.Contains(body, "faked "+id) {
			t.Errorf("output should name %s:\n%s", id, body)
		}
	}
	if handle.Migrator().HasTable("widgets") {
		t.Error("faking must not create tables")
	}

	runner := migrate.NewRunner(handle, "sqlite", pool)
	pending, err := runner.Pending(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("%d migrations are still pending after faking", len(pending))
	}
}

func TestFakeInitialNeedsTheTablesToExist(t *testing.T) {
	app, handle, pool := fakeMigrateApp(t)
	if err := migrate.Default().Add(pool...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { migrate.Reset() })

	if _, err := runMigrate(t, app, cli.Migrate{FakeInitial: true}); err == nil {
		t.Fatal("--fake-initial should refuse when the tables are absent")
	}

	if err := handle.Exec(`CREATE TABLE widgets (id text primary key, label text)`).Error; err != nil {
		t.Fatal(err)
	}

	body, err := runMigrate(t, app, cli.Migrate{FakeInitial: true})
	if err != nil {
		t.Fatalf("with the tables present it should succeed: %v", err)
	}
	if !strings.Contains(body, "faked 0001_initial") {
		t.Errorf("output = %s", body)
	}
	if strings.Contains(body, "0002_add_stock") {
		t.Error("--fake-initial should only record the first migration")
	}
}

func TestMigrateToStopsAtATarget(t *testing.T) {
	app, handle, pool := fakeMigrateApp(t)
	if err := migrate.Default().Add(pool...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { migrate.Reset() })

	body, err := runMigrate(t, app, cli.Migrate{Target: "0001_initial"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "applied 1 migration") {
		t.Errorf("only the target should be applied:\n%s", body)
	}
	if !handle.Migrator().HasTable("widgets") {
		t.Error("the first migration should have run")
	}
	if handle.Migrator().HasColumn("widgets", "stock") {
		t.Error("the second migration should not have run")
	}

	if _, err := runMigrate(t, app, cli.Migrate{Target: "0009_nope"}); err == nil {
		t.Error("an unknown target should be an error")
	}
}
