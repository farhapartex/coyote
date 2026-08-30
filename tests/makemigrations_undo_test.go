package tests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

func undoFixture(t *testing.T) (*fakeApp, string, migrate.Snapshot) {
	t.Helper()
	migrate.Reset()
	t.Cleanup(migrate.Reset)

	dir := t.TempDir()
	handle := newTestDB(t)
	before := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})
	after := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widgetV2{})})

	change := migrate.Diff(before, after)
	generated, err := migrate.Generate(dir, "add_quantity", change)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.SaveSnapshot(dir, after); err != nil {
		t.Fatal(err)
	}
	migrate.Register(migrate.Migration{ID: generated.ID, Note: "add_quantity", Up: change.Ops})

	cfg := devSettings(t, func(s *settings.Settings) { s.Migrations.Dir = dir })
	return &fakeApp{cfg: cfg, handle: handle}, dir, before
}

func TestMakeMigrationsUndoRewindsAnUnappliedMigration(t *testing.T) {
	app, dir, before := undoFixture(t)
	pool := migrate.Registered()
	path := filepath.Join(dir, pool[0].ID+".go")

	out := &bytes.Buffer{}
	command := cli.MakeMigrations{Undo: true, NoInput: true}
	if err := command.Run(cli.Context{App: app, Out: out}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the migration file should be gone, stat gave %v", err)
	}
	rewound, err := migrate.LoadSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if change := migrate.Diff(rewound, before); !change.Empty() {
		t.Errorf("the snapshot should be back where it started, got %d ops", len(change.Ops))
	}
	if !strings.Contains(out.String(), "rewound") {
		t.Errorf("undo should say what it did, got %q", out.String())
	}
}

func TestMakeMigrationsUndoRefusesAnAppliedMigration(t *testing.T) {
	app, dir, _ := undoFixture(t)
	pool := migrate.Registered()
	path := filepath.Join(dir, pool[0].ID+".go")

	if err := app.handle.AutoMigrate(&widget{}); err != nil {
		t.Fatal(err)
	}

	runner := migrate.NewRunner(app.handle, app.cfg.Database().Engine, pool)
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply(background, pool[0]); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	command := cli.MakeMigrations{Undo: true, NoInput: true}
	err := command.Run(cli.Context{App: app, Out: out})
	if err == nil {
		t.Fatal("undo must refuse a migration that has been applied")
	}
	if !strings.Contains(err.Error(), "rollback") {
		t.Errorf("the refusal should point at rollback, got %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the migration file must survive a refused undo, stat gave %v", statErr)
	}
}

func TestMakeMigrationsUndoLeavesEverythingAloneWithoutConfirmation(t *testing.T) {
	app, dir, _ := undoFixture(t)
	pool := migrate.Registered()
	path := filepath.Join(dir, pool[0].ID+".go")
	beforeSnapshot, err := migrate.LoadSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	command := cli.MakeMigrations{Undo: true}
	if err := command.Run(cli.Context{App: app, Out: out, In: strings.NewReader("no\n")}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("declining should keep the migration file, stat gave %v", err)
	}
	after, err := migrate.LoadSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if change := migrate.Diff(after, beforeSnapshot); !change.Empty() {
		t.Error("declining should leave the snapshot alone")
	}
	if !strings.Contains(out.String(), "left alone") {
		t.Errorf("undo should say it did nothing, got %q", out.String())
	}
}

func TestMakeMigrationsUndoWithNothingToUndo(t *testing.T) {
	migrate.Reset()
	t.Cleanup(migrate.Reset)

	dir := t.TempDir()
	cfg := devSettings(t, func(s *settings.Settings) { s.Migrations.Dir = dir })
	app := &fakeApp{cfg: cfg, handle: newTestDB(t)}

	out := &bytes.Buffer{}
	command := cli.MakeMigrations{Undo: true, NoInput: true}
	if err := command.Run(cli.Context{App: app, Out: out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing to undo") {
		t.Errorf("undo with an empty ladder should say so, got %q", out.String())
	}
}
