package tests

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/contrib/migrate/dialect"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

func relatedSnapshot(t *testing.T, handle *gorm.DB) migrate.Snapshot {
	t.Helper()
	return migrate.SnapshotOf([]*model.Schema{
		schemaFor(t, handle, Category{}),
		schemaFor(t, handle, Item{}),
	})
}

func TestBelongsToReachesTheSnapshot(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)

	items, ok := desired.Table("items")
	if !ok {
		t.Fatal("items should be in the snapshot")
	}
	column, ok := items.Column("category_id")
	if !ok {
		t.Fatal("items.category_id should be in the snapshot")
	}
	if column.References.Table != "categories" || column.References.Column != "id" {
		t.Errorf("references = %+v, want categories(id)", column.References)
	}

	title, ok := items.Column("title")
	if !ok {
		t.Fatal("items.title should be in the snapshot")
	}
	if !title.References.IsZero() {
		t.Errorf("a plain column should carry no reference, got %+v", title.References)
	}
}

func TestGeneratedDDLDeclaresForeignKeys(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)
	items, _ := desired.Table("items")
	op := migrate.CreateTable{Table: items}

	for _, c := range []struct {
		engine settings.Engine
		want   []string
	}{
		{settings.SQLite, []string{`FOREIGN KEY ("category_id")`, `REFERENCES "categories" ("id")`}},
		{settings.Postgres, []string{`FOREIGN KEY ("category_id")`, `REFERENCES "categories" ("id")`}},
		{settings.MySQL, []string{"FOREIGN KEY (`category_id`)", "REFERENCES `categories` (`id`)"}},
	} {
		joined := strings.Join(op.Statements(dialect.For(c.engine)), "\n")
		for _, want := range c.want {
			if !strings.Contains(joined, want) {
				t.Errorf("%s DDL missing %q:\n%s", c.engine, want, joined)
			}
		}
	}
}

func TestAddedRelationColumnDeclaresItsForeignKey(t *testing.T) {
	op := migrate.AddColumn{
		Table: "items",
		Column: migrate.Column{
			Name:       "category_id",
			Kind:       model.KindString,
			Size:       64,
			References: migrate.Reference{Table: "categories", Column: "id"},
		},
	}

	for _, engine := range []settings.Engine{settings.SQLite, settings.Postgres, settings.MySQL} {
		statements := op.Statements(dialect.For(engine))
		joined := strings.Join(statements, "\n")
		if !strings.Contains(joined, "REFERENCES") {
			t.Errorf("%s add column missing the reference:\n%s", engine, joined)
		}
		if !strings.Contains(joined, "category_id") {
			t.Errorf("%s add column missing the column:\n%s", engine, joined)
		}
	}

	mysql := op.Statements(dialect.For(settings.MySQL))
	if len(mysql) != 2 {
		t.Fatalf("MySQL needs a separate ADD CONSTRAINT, got %d statement(s): %v", len(mysql), mysql)
	}
	if !strings.Contains(mysql[1], "ADD CONSTRAINT") {
		t.Errorf("MySQL ignores an inline REFERENCES, so the second statement must add the constraint, got %q", mysql[1])
	}
}

func TestForeignKeyIsEnforcedAfterMigrating(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)
	change := migrate.Diff(migrate.Snapshot{Version: 1}, desired)

	migrate.Reset()
	t.Cleanup(migrate.Reset)
	migrate.Register(migrate.Migration{ID: "0001_initial", Up: change.Ops})

	runner := migrate.NewRunner(handle, settings.SQLite, migrate.Registered())
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		t.Fatal(err)
	}
	pending, err := runner.Pending(background)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range pending {
		if err := runner.Apply(background, entry); err != nil {
			t.Fatal(err)
		}
	}

	var enabled int
	if err := handle.Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("this test is meaningless without PRAGMA foreign_keys, got %d", enabled)
	}

	if err := handle.Exec(`INSERT INTO categories (id, name) VALUES ('c1', 'Kitchen')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := handle.Exec(`INSERT INTO items (id, title, category_id) VALUES ('i1', 'Pan', 'c1')`).Error; err != nil {
		t.Errorf("a row pointing at a real category should be accepted: %v", err)
	}

	err = handle.Exec(`INSERT INTO items (id, title, category_id) VALUES ('i2', 'Ghost', 'no-such-category')`).Error
	if err == nil {
		t.Fatal("a row pointing at a category that does not exist must be refused")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Errorf("the refusal should name the constraint, got %v", err)
	}

	if err := handle.Exec(`INSERT INTO items (id, title, category_id) VALUES ('i3', 'Loose', NULL)`).Error; err != nil {
		t.Errorf("a nullable relation should still accept NULL: %v", err)
	}
}

func TestRebuiltTableKeepsItsForeignKeys(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)
	items, _ := desired.Table("items")

	widened := items
	widened.Columns = nil
	for _, column := range items.Columns {
		if column.Name == "title" {
			column.Size = 250
		}
		widened.Columns = append(widened.Columns, column)
	}

	from, _ := items.Column("title")
	to, _ := widened.Column("title")
	op := migrate.AlterColumn{
		Table:   "items",
		From:    from,
		To:      to,
		Columns: widened.Columns,
		Indexes: widened.Indexes,
	}

	joined := strings.Join(op.Statements(dialect.For(settings.SQLite)), "\n")
	if !strings.Contains(joined, `REFERENCES "categories" ("id")`) {
		t.Errorf("a SQLite table rebuild must carry the foreign keys forward:\n%s", joined)
	}
	if strings.Contains(joined, "items__rebuilt_category_id") {
		t.Errorf("the constraint should be named after the real table, not the shadow:\n%s", joined)
	}
}

func TestForeignKeyChangeIsDetectedAndApplied(t *testing.T) {
	before := migrate.Table{
		Name: "items",
		Columns: []migrate.Column{
			{Name: "id", Kind: model.KindString, Size: 64, PrimaryKey: true, NotNull: true},
			{Name: "category_id", Kind: model.KindString, Size: 64},
		},
	}
	after := migrate.Table{
		Name: "items",
		Columns: []migrate.Column{
			{Name: "id", Kind: model.KindString, Size: 64, PrimaryKey: true, NotNull: true},
			{Name: "category_id", Kind: model.KindString, Size: 64,
				References: migrate.Reference{Table: "categories", Column: "id"}},
		},
	}

	change := migrate.Diff(
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{before}},
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{after}},
	)
	if change.Empty() {
		t.Fatal("adding a foreign key to an existing column should produce a change")
	}

	for _, engine := range []settings.Engine{settings.Postgres, settings.MySQL} {
		joined := ""
		for _, op := range change.Ops {
			if statementer, ok := op.(interface {
				Statements(dialect.Dialect) []string
			}); ok {
				joined += strings.Join(statementer.Statements(dialect.For(engine)), "\n")
			}
		}
		if !strings.Contains(joined, "ADD CONSTRAINT") {
			t.Errorf("%s should add the constraint:\n%s", engine, joined)
		}
	}

	reverse := migrate.Diff(
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{after}},
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{before}},
	)
	joined := ""
	for _, op := range reverse.Ops {
		if statementer, ok := op.(interface {
			Statements(dialect.Dialect) []string
		}); ok {
			joined += strings.Join(statementer.Statements(dialect.For(settings.Postgres)), "\n")
		}
	}
	if !strings.Contains(joined, "DROP CONSTRAINT") {
		t.Errorf("removing a foreign key should drop the constraint:\n%s", joined)
	}
}

func createOrder(t *testing.T, change migrate.Change) []string {
	t.Helper()
	out := []string{}
	for _, op := range change.Ops {
		if created, ok := op.(migrate.CreateTable); ok {
			out = append(out, created.Table.Name)
		}
	}
	return out
}

func TestTablesAreCreatedAfterWhatTheyReference(t *testing.T) {
	orders := migrate.Table{Name: "orders", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
		{Name: "user_id", Kind: model.KindString, References: migrate.Reference{Table: "users", Column: "id"}},
	}}
	users := migrate.Table{Name: "users", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
	}}
	lines := migrate.Table{Name: "order_lines", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
		{Name: "order_id", Kind: model.KindString, References: migrate.Reference{Table: "orders", Column: "id"}},
	}}

	change := migrate.Diff(migrate.Snapshot{Version: 1},
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{lines, orders, users}})

	order := createOrder(t, change)
	position := map[string]int{}
	for i, name := range order {
		position[name] = i
	}
	if position["users"] > position["orders"] {
		t.Errorf("users must be created before orders, got %v", order)
	}
	if position["orders"] > position["order_lines"] {
		t.Errorf("orders must be created before order_lines, got %v", order)
	}
}

func TestTablesAreDroppedBeforeWhatTheyReference(t *testing.T) {
	orders := migrate.Table{Name: "orders", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
		{Name: "user_id", Kind: model.KindString, References: migrate.Reference{Table: "users", Column: "id"}},
	}}
	users := migrate.Table{Name: "users", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
	}}

	change := migrate.Diff(
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{orders, users}},
		migrate.Snapshot{Version: 1},
	)

	dropped := []string{}
	for _, op := range change.Ops {
		if drop, ok := op.(migrate.DropTable); ok {
			dropped = append(dropped, drop.Name)
		}
	}
	if len(dropped) != 2 {
		t.Fatalf("both tables should be dropped, got %v", dropped)
	}
	if dropped[0] != "orders" {
		t.Errorf("the child must be dropped first, got %v", dropped)
	}
}

func TestACycleOfReferencesStillProducesEveryTable(t *testing.T) {
	a := migrate.Table{Name: "a", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
		{Name: "b_id", Kind: model.KindString, References: migrate.Reference{Table: "b", Column: "id"}},
	}}
	b := migrate.Table{Name: "b", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
		{Name: "a_id", Kind: model.KindString, References: migrate.Reference{Table: "a", Column: "id"}},
	}}
	selfRef := migrate.Table{Name: "nodes", Columns: []migrate.Column{
		{Name: "id", Kind: model.KindString, PrimaryKey: true, NotNull: true},
		{Name: "parent_id", Kind: model.KindString, References: migrate.Reference{Table: "nodes", Column: "id"}},
	}}

	change := migrate.Diff(migrate.Snapshot{Version: 1},
		migrate.Snapshot{Version: 1, Tables: []migrate.Table{a, b, selfRef}})

	order := createOrder(t, change)
	if len(order) != 3 {
		t.Errorf("a cycle must not drop a table from the plan, got %v", order)
	}
}

func TestGeneratedMigrationSourceCarriesTheReference(t *testing.T) {
	handle := newTestDB(t)
	dir := t.TempDir()
	desired := relatedSnapshot(t, handle)

	generated, err := migrate.Generate(dir, "initial", migrate.Diff(migrate.Snapshot{Version: 1}, desired))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(generated.Path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)

	if !strings.Contains(source, `References: migrate.Reference{Table: "categories", Column: "id"}`) {
		t.Errorf("the generated migration should declare the reference:\n%s", source)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), generated.Path, raw, parser.AllErrors); err != nil {
		t.Errorf("the generated migration should parse: %v", err)
	}
}

func TestRebuildRefusesToRecordWithADanglingChild(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)
	initial := migrate.Diff(migrate.Snapshot{Version: 1}, desired)

	items, _ := desired.Table("items")
	widened := migrate.Table{Name: items.Name, Indexes: items.Indexes}
	for _, column := range items.Columns {
		if column.Name == "title" {
			column.Size = 250
		}
		widened.Columns = append(widened.Columns, column)
	}
	from, _ := items.Column("title")
	to, _ := widened.Column("title")
	rebuild := migrate.AlterColumn{
		Table: "items", From: from, To: to,
		Columns: widened.Columns, Indexes: widened.Indexes,
	}

	migrate.Reset()
	t.Cleanup(migrate.Reset)
	migrate.Register(
		migrate.Migration{ID: "0001_initial", Up: initial.Ops},
		migrate.Migration{ID: "0002_widen_title", Up: []migrate.Op{rebuild}},
	)

	runner := migrate.NewRunner(handle, settings.SQLite, migrate.Registered())
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		t.Fatal(err)
	}
	pool := migrate.Registered()
	if err := runner.Apply(background, pool[0]); err != nil {
		t.Fatal(err)
	}

	if err := handle.Exec(`INSERT INTO categories (id, name) VALUES ('c1', 'Kitchen')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := handle.Exec(`INSERT INTO items (id, title, category_id) VALUES ('i1', 'Pan', 'c1')`).Error; err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"PRAGMA foreign_keys = OFF",
		`DELETE FROM categories WHERE id = 'c1'`,
		"PRAGMA foreign_keys = ON",
	} {
		if err := handle.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	err := runner.Apply(background, pool[1])
	if err == nil {
		t.Fatal("a rebuild that leaves a dangling child must not be recorded")
	}
	if !strings.Contains(err.Error(), "foreign key violation") {
		t.Errorf("the failure should name the violation, got %v", err)
	}

	applied, err := runner.Applied(background)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range applied {
		if entry.ID == "0002_widen_title" {
			t.Error("the failed rebuild must not appear in the ledger")
		}
	}
}

func TestRollingBackARebuildOfAReferencedTable(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)
	initial := migrate.Diff(migrate.Snapshot{Version: 1}, desired)

	categories, _ := desired.Table("categories")
	widened := migrate.Table{Name: categories.Name, Indexes: categories.Indexes}
	for _, column := range categories.Columns {
		if column.Name == "name" {
			column.Size = 250
		}
		widened.Columns = append(widened.Columns, column)
	}
	from, _ := categories.Column("name")
	to, _ := widened.Column("name")
	rebuild := migrate.AlterColumn{
		Table: "categories", From: from, To: to,
		Columns: widened.Columns, Indexes: widened.Indexes,
	}

	migrate.Reset()
	t.Cleanup(migrate.Reset)
	migrate.Register(
		migrate.Migration{ID: "0001_initial", Up: initial.Ops},
		migrate.Migration{ID: "0002_widen_name", Up: []migrate.Op{rebuild}},
	)

	runner := migrate.NewRunner(handle, settings.SQLite, migrate.Registered())
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		t.Fatal(err)
	}
	for _, entry := range migrate.Registered() {
		if err := runner.Apply(background, entry); err != nil {
			t.Fatalf("applying %s: %v", entry.ID, err)
		}
	}

	if err := handle.Exec(`INSERT INTO categories (id, name) VALUES ('c1', 'Kitchen')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := handle.Exec(`INSERT INTO items (id, title, category_id) VALUES ('i1', 'Pan', 'c1')`).Error; err != nil {
		t.Fatal(err)
	}

	plan, err := runner.PlanRollback(background, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 {
		t.Fatalf("expected one migration to roll back, got %d", len(plan))
	}
	if err := runner.Undo(background, plan[0]); err != nil {
		t.Fatalf("rolling back a rebuild of a referenced table should work: %v", err)
	}

	var rows int64
	if err := handle.Raw(`SELECT count(*) FROM items`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("the child rows should survive the rollback, got %d", rows)
	}
	if err := handle.Raw(`SELECT count(*) FROM categories`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("the parent rows should survive the rollback, got %d", rows)
	}

	err = handle.Exec(`INSERT INTO items (id, title, category_id) VALUES ('i9', 'Ghost', 'nope')`).Error
	if err == nil {
		t.Error("the foreign key should still be enforced after a rollback")
	}
}

func TestARebuildRefusesWhenTheDataIsAlreadyBroken(t *testing.T) {
	handle := newTestDB(t)
	desired := relatedSnapshot(t, handle)
	initial := migrate.Diff(migrate.Snapshot{Version: 1}, desired)

	items, _ := desired.Table("items")
	widened := migrate.Table{Name: items.Name, Indexes: items.Indexes}
	for _, column := range items.Columns {
		if column.Name == "title" {
			column.Size = 250
		}
		widened.Columns = append(widened.Columns, column)
	}
	from, _ := items.Column("title")
	to, _ := widened.Column("title")

	migrate.Reset()
	t.Cleanup(migrate.Reset)
	migrate.Register(
		migrate.Migration{ID: "0001_initial", Up: initial.Ops},
		migrate.Migration{ID: "0002_widen_title", Up: []migrate.Op{migrate.AlterColumn{
			Table: "items", From: from, To: to,
			Columns: widened.Columns, Indexes: widened.Indexes,
		}}},
	)

	runner := migrate.NewRunner(handle, settings.SQLite, migrate.Registered())
	background := context.Background()
	if err := runner.Prepare(background); err != nil {
		t.Fatal(err)
	}
	pool := migrate.Registered()
	if err := runner.Apply(background, pool[0]); err != nil {
		t.Fatal(err)
	}

	for _, statement := range []string{
		"PRAGMA foreign_keys = OFF",
		`INSERT INTO items (id, title, category_id) VALUES ('i1', 'Ghost', 'no-such-category')`,
		"PRAGMA foreign_keys = ON",
	} {
		if err := handle.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	err := runner.Apply(background, pool[1])
	if err == nil {
		t.Fatal("a rebuild must refuse to run over data that already violates a foreign key")
	}
	if !strings.Contains(err.Error(), "already holds") {
		t.Errorf("the refusal should say the data was broken before the rebuild, got %v", err)
	}

	var width string
	if err := handle.Raw(`SELECT sql FROM sqlite_master WHERE name = 'items'`).Scan(&width).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(width, "250") {
		t.Error("a refused rebuild must leave the table alone")
	}
}
