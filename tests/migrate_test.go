package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/db"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

type widget struct {
	ID    string `gorm:"primaryKey"`
	Label string
}

type widgetV2 struct {
	ID       string `gorm:"primaryKey"`
	Label    string
	Quantity int
}

func (widgetV2) TableName() string { return "widgets" }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := settings.Database{
		Alias:  "default",
		Engine: settings.SQLite,
		Name:   filepath.Join(t.TempDir(), "test.db"),
	}
	handle, err := db.Open(cfg, db.Options{})
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { db.Close(handle) })
	return handle
}

func TestMigratePlanReportsMissingTable(t *testing.T) {
	handle := newTestDB(t)
	runner := migrate.New(handle, []model.Model{model.Of(widget{})})

	report, err := runner.Plan()
	if err != nil {
		t.Fatal(err)
	}
	if report.Empty() {
		t.Fatal("a fresh database should have pending changes")
	}
	if len(report.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(report.Changes))
	}
	change := report.Changes[0]
	if change.Table != "widgets" {
		t.Errorf("table = %q, want widgets (gorm naming)", change.Table)
	}
	if !change.Created {
		t.Error("change should be a table creation")
	}
	if !strings.Contains(change.String(), "create table widgets") {
		t.Errorf("unexpected description: %q", change.String())
	}
}

func TestMigrateAppliesAndIsIdempotent(t *testing.T) {
	handle := newTestDB(t)
	runner := migrate.New(handle, []model.Model{model.Of(widget{})})

	report, err := runner.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Changes) != 1 {
		t.Fatalf("expected 1 applied change, got %d", len(report.Changes))
	}
	if !handle.Migrator().HasTable(&widget{}) {
		t.Fatal("table was not created")
	}

	again, err := runner.Plan()
	if err != nil {
		t.Fatal(err)
	}
	if !again.Empty() {
		t.Errorf("second plan should be empty, got %v", again.Tables())
	}

	third, err := runner.Apply()
	if err != nil {
		t.Fatalf("re-applying should be a no-op: %v", err)
	}
	if !third.Empty() {
		t.Error("re-applying should report no changes")
	}
}

func TestMigrateDetectsNewColumn(t *testing.T) {
	handle := newTestDB(t)

	first := migrate.New(handle, []model.Model{model.Of(widget{})})
	if _, err := first.Apply(); err != nil {
		t.Fatal(err)
	}

	second := migrate.New(handle, []model.Model{model.Of(widgetV2{})})
	report, err := second.Plan()
	if err != nil {
		t.Fatal(err)
	}
	if report.Empty() {
		t.Fatal("adding a field should produce a pending change")
	}
	change := report.Changes[0]
	if change.Created {
		t.Error("table already exists, so this is an alteration")
	}
	if len(change.AddedColumns) != 1 || change.AddedColumns[0] != "quantity" {
		t.Errorf("added columns = %v, want [quantity]", change.AddedColumns)
	}

	if _, err := second.Apply(); err != nil {
		t.Fatal(err)
	}
	if !handle.Migrator().HasColumn(&widgetV2{}, "quantity") {
		t.Error("column was not added")
	}
}

func TestMigrateCreatesUserTableFromEntity(t *testing.T) {
	handle := newTestDB(t)
	runner := migrate.New(handle, []model.Model{model.Of(auth.User{})})

	if _, err := runner.Apply(); err != nil {
		t.Fatal(err)
	}
	if !handle.Migrator().HasTable(&auth.User{}) {
		t.Fatal("users table was not created")
	}

	for _, column := range []string{
		"id", "first_name", "last_name", "email", "username", "password",
		"is_active", "is_superadmin", "last_login_at", "created_at", "updated_at",
	} {
		if !handle.Migrator().HasColumn(&auth.User{}, column) {
			t.Errorf("missing column %q", column)
		}
	}
	if !handle.Migrator().HasIndex(&auth.User{}, "Username") {
		t.Error("expected the unique index on username")
	}
}

func TestModelRegistry(t *testing.T) {
	registry := model.NewRegistry(model.Of(widget{}))
	if registry.Len() != 1 {
		t.Fatalf("Len = %d", registry.Len())
	}
	registry.Add(model.Named("things", widgetV2{}))
	if registry.Len() != 2 {
		t.Fatalf("Len = %d", registry.Len())
	}
	if got := registry.All()[1].Table; got != "things" {
		t.Errorf("table override = %q", got)
	}
	if len(registry.Entities()) != 2 {
		t.Error("Entities should mirror the models")
	}
}

func TestDBRejectsUnsupportedEngine(t *testing.T) {
	_, err := db.Open(settings.Database{Alias: "x", Engine: settings.Postgres, Name: "db", Host: "h", Port: 5432}, db.Options{})
	if err == nil {
		t.Fatal("expected an error for an engine with no bundled driver")
	}
	if !strings.Contains(err.Error(), "unsupported engine") {
		t.Errorf("error = %v", err)
	}
}
