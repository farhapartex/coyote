package tests

import (
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/model"
)

func TestSnapshotApplyReplaysAGeneratedChange(t *testing.T) {
	handle := newTestDB(t)
	before := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})
	after := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widgetV2{})})

	change := migrate.Diff(before, after)
	if change.Empty() {
		t.Fatal("adding a column should produce a change")
	}

	forward, err := before.Apply(change.Ops)
	if err != nil {
		t.Fatal(err)
	}
	if replayed := migrate.Diff(forward, after); !replayed.Empty() {
		t.Errorf("replaying the change should land on the new snapshot, got %d ops", len(replayed.Ops))
	}
}

func TestSnapshotApplyRewindsAGeneratedChange(t *testing.T) {
	handle := newTestDB(t)
	before := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})
	after := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widgetV2{})})

	change := migrate.Diff(before, after)
	inverse, blocked := migrate.Invert(change.Ops)
	if len(blocked) > 0 {
		t.Fatalf("the generated change should reverse, blocked on %v", blocked)
	}

	rewound, err := after.Apply(inverse)
	if err != nil {
		t.Fatal(err)
	}
	if change := migrate.Diff(rewound, before); !change.Empty() {
		t.Errorf("rewinding should land back on the previous snapshot, got %d ops", len(change.Ops))
	}
	if change := migrate.Diff(before, rewound); !change.Empty() {
		t.Errorf("the rewound snapshot should be indistinguishable from the previous one, got %d ops", len(change.Ops))
	}
}

func TestSnapshotApplyLeavesTheReceiverAlone(t *testing.T) {
	handle := newTestDB(t)
	before := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})
	columns := len(before.Tables[0].Columns)

	if _, err := before.Apply([]migrate.Op{
		migrate.AddColumn{Table: "widgets", Column: migrate.Column{Name: "quantity", Kind: model.KindInt}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(before.Tables[0].Columns); got != columns {
		t.Errorf("Apply mutated its receiver: %d columns, want %d", got, columns)
	}
}

func TestSnapshotApplyRefusesWhatItCannotReplay(t *testing.T) {
	handle := newTestDB(t)
	base := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})

	cases := map[string][]migrate.Op{
		"cannot be replayed": {migrate.RunSQL{Any: "UPDATE widgets SET label = 'x'"}},
		"is not in the snapshot": {
			migrate.DropColumn{Table: "widgets", Column: "nosuchcolumn"},
		},
		"widgets is already in the snapshot": {
			migrate.CreateTable{Table: migrate.Table{Name: "widgets"}},
		},
	}
	for want, ops := range cases {
		if _, err := base.Apply(ops); err == nil {
			t.Errorf("applying %v should fail with a message mentioning %q", ops, want)
		} else if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err.Error(), want)
		}
	}
}

func TestSnapshotApplyBuildsATableFromNothing(t *testing.T) {
	handle := newTestDB(t)
	desired := migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, widget{})})

	empty := migrate.Snapshot{Version: 1}
	change := migrate.Diff(empty, desired)

	built, err := empty.Apply(change.Ops)
	if err != nil {
		t.Fatal(err)
	}
	if change := migrate.Diff(built, desired); !change.Empty() {
		t.Errorf("creating a table should land on the desired snapshot, got %d ops", len(change.Ops))
	}

	inverse, blocked := migrate.Invert(change.Ops)
	if len(blocked) > 0 {
		t.Fatalf("creating a table should reverse, blocked on %v", blocked)
	}
	back, err := built.Apply(inverse)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Tables) != 0 {
		t.Errorf("dropping the table should empty the snapshot, got %d tables", len(back.Tables))
	}
}

func TestSnapshotApplyHandlesIndexesAndRenames(t *testing.T) {
	base := migrate.Snapshot{Version: 1, Tables: []migrate.Table{{
		Name:    "widgets",
		Columns: []migrate.Column{{Name: "id", Kind: model.KindString, PrimaryKey: true}, {Name: "label", Kind: model.KindString}},
	}}}

	ops := []migrate.Op{
		migrate.CreateIndex{Table: "widgets", Name: "idx_widgets_label", Columns: []string{"label"}, Unique: true},
		migrate.RenameColumn{Table: "widgets", From: "label", To: "title"},
	}
	next, err := base.Apply(ops)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := next.Tables[0].Index("idx_widgets_label"); !ok {
		t.Error("the index should be in the snapshot")
	}
	if _, ok := next.Tables[0].Column("title"); !ok {
		t.Error("the column should have been renamed")
	}
	if _, ok := next.Tables[0].Column("label"); ok {
		t.Error("the old column name should be gone")
	}

	inverse, blocked := migrate.Invert(ops)
	if len(blocked) > 0 {
		t.Fatalf("blocked on %v", blocked)
	}
	back, err := next.Apply(inverse)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Tables[0].Indexes) != 0 {
		t.Error("the index should be gone again")
	}
	if _, ok := back.Tables[0].Column("label"); !ok {
		t.Error("the rename should have been undone")
	}
}
