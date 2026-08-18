package tests

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
)

func rollbackApp(t *testing.T) (*fakeApp, *gorm.DB, []migrate.Migration) {
	t.Helper()
	handle := newTestDB(t)

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
			Up: []migrate.Op{
				migrate.AddColumn{Table: "widgets", Column: migrate.Column{Name: "stock", Kind: model.KindInt}},
				migrate.CreateIndex{Table: "widgets", Name: "idx_widgets_stock", Columns: []string{"stock"}},
			},
		},
	}

	migrate.Reset()
	if err := migrate.Default().Add(pool...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(migrate.Reset)

	app := &fakeApp{cfg: devSettings(t), handle: handle}
	if _, err := runMigrate(t, app, cli.Migrate{}); err != nil {
		t.Fatal(err)
	}
	return app, handle, pool
}

func runRollback(t *testing.T, app *fakeApp, command cli.Rollback, answer string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	err := command.Run(cli.Context{App: app, Out: out, In: strings.NewReader(answer)})
	return out.String(), err
}

func TestRollbackUndoesTheLastMigration(t *testing.T) {
	app, handle, _ := rollbackApp(t)

	if !handle.Migrator().HasColumn("widgets", "stock") {
		t.Fatal("setup did not apply the second migration")
	}

	body, err := runRollback(t, app, cli.Rollback{Steps: 1, NoInput: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "rolled back 0002_add_stock") {
		t.Errorf("output = %s", body)
	}

	if handle.Migrator().HasColumn("widgets", "stock") {
		t.Error("the added column should be gone")
	}
	if !handle.Migrator().HasTable("widgets") {
		t.Error("the first migration should still be applied")
	}

	runner := migrate.NewRunner(handle, "sqlite", migrate.Registered())
	pending, err := runner.Pending(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != "0002_add_stock" {
		t.Errorf("the rolled-back migration should be pending again: %+v", pending)
	}
}

func TestRollbackRunsOpsInReverseOrder(t *testing.T) {
	app, handle, _ := rollbackApp(t)

	if _, err := runRollback(t, app, cli.Rollback{Steps: 1, NoInput: true}, ""); err != nil {
		t.Fatal(err)
	}

	indexes, err := handle.Migrator().GetIndexes("widgets")
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if index.Name() == "idx_widgets_stock" {
			t.Error("the index should have been dropped before its column")
		}
	}
}

func TestRollbackUndoesSeveralSteps(t *testing.T) {
	app, handle, _ := rollbackApp(t)

	body, err := runRollback(t, app, cli.Rollback{Steps: 2, NoInput: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "rolled back 2 migration") {
		t.Errorf("output = %s", body)
	}
	if handle.Migrator().HasTable("widgets") {
		t.Error("both migrations should be undone")
	}
}

func TestRollbackWarnsAndAsksBeforeDestroyingData(t *testing.T) {
	app, handle, _ := rollbackApp(t)

	body, err := runRollback(t, app, cli.Rollback{Steps: 1}, "no\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "this will destroy") {
		t.Errorf("a destructive rollback should say so:\n%s", body)
	}
	if !strings.Contains(body, "every value in widgets.stock") {
		t.Errorf("it should name what is lost:\n%s", body)
	}
	if !strings.Contains(body, "left alone") {
		t.Errorf("answering no should abandon the rollback:\n%s", body)
	}
	if !handle.Migrator().HasColumn("widgets", "stock") {
		t.Error("nothing should have been dropped")
	}

	if _, err := runRollback(t, app, cli.Rollback{Steps: 1}, "yes\n"); err != nil {
		t.Fatal(err)
	}
	if handle.Migrator().HasColumn("widgets", "stock") {
		t.Error("answering yes should roll back")
	}
}

func TestRollbackRefusesWhatItCannotReverse(t *testing.T) {
	handle := newTestDB(t)
	migrate.Reset()
	t.Cleanup(migrate.Reset)

	if err := migrate.Default().Add(migrate.Migration{
		ID: "0001_data",
		Up: []migrate.Op{migrate.RunSQL{Any: "SELECT 1", Note: "a data step"}},
	}); err != nil {
		t.Fatal(err)
	}

	app := &fakeApp{cfg: devSettings(t), handle: handle}
	if _, err := runMigrate(t, app, cli.Migrate{}); err != nil {
		t.Fatal(err)
	}

	body, err := runRollback(t, app, cli.Rollback{Steps: 1, NoInput: true}, "")
	if err == nil {
		t.Fatal("an irreversible migration should refuse to roll back")
	}
	if !strings.Contains(err.Error(), "cannot be reversed") {
		t.Errorf("error = %v", err)
	}
	if !strings.Contains(body, "cannot reverse") {
		t.Errorf("the plan should show which op is the problem:\n%s", body)
	}
}

func TestExplicitDownWins(t *testing.T) {
	handle := newTestDB(t)
	migrate.Reset()
	t.Cleanup(migrate.Reset)

	if err := migrate.Default().Add(migrate.Migration{
		ID: "0001_data",
		Up: []migrate.Op{migrate.RunSQL{Any: "CREATE TABLE notes (id text)", Note: "by hand"}},
		Down: []migrate.Op{
			migrate.RunSQL{Any: "DROP TABLE notes", Note: "by hand"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	app := &fakeApp{cfg: devSettings(t), handle: handle}
	if _, err := runMigrate(t, app, cli.Migrate{}); err != nil {
		t.Fatal(err)
	}
	if !handle.Migrator().HasTable("notes") {
		t.Fatal("setup failed")
	}

	if _, err := runRollback(t, app, cli.Rollback{Steps: 1, NoInput: true}, ""); err != nil {
		t.Fatalf("an explicit Down should be used: %v", err)
	}
	if handle.Migrator().HasTable("notes") {
		t.Error("the explicit Down should have dropped the table")
	}
}

func TestNothingToRollBack(t *testing.T) {
	handle := newTestDB(t)
	migrate.Reset()
	t.Cleanup(migrate.Reset)

	app := &fakeApp{cfg: devSettings(t), handle: handle}
	body, err := runRollback(t, app, cli.Rollback{Steps: 1, NoInput: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "nothing to roll back") {
		t.Errorf("output = %s", body)
	}
}

func TestGeneratedFilesCarryADown(t *testing.T) {
	handle := newTestDB(t)
	dir := t.TempDir()

	desired := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})
	generated, err := migrate.Generate(dir, "create widgets", migrate.Diff(migrate.Snapshot{}, desired))
	if err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(generated.Path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)

	if !strings.Contains(source, "Down: []migrate.Op{") {
		t.Errorf("a generated migration should carry its reverse:\n%s", source)
	}
	if !strings.Contains(source, "migrate.DropTable{") {
		t.Errorf("the reverse of a create should be a drop:\n%s", source)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), generated.Path, nil, parser.AllErrors); err != nil {
		t.Errorf("the file with a Down should still parse: %v", err)
	}
}

func TestIrreversibleOpsAreNotedInTheFile(t *testing.T) {
	dir := t.TempDir()
	change := migrate.Change{Ops: []migrate.Op{
		migrate.RunSQL{Any: "UPDATE widgets SET label = 'x'", Note: "backfill"},
	}}

	generated, err := migrate.Generate(dir, "backfill", change)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(generated.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "no automatic reverse for") {
		t.Errorf("the generator should say what it could not reverse:\n%s", body)
	}
}
