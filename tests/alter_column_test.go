package tests

import (
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/contrib/migrate/dialect"
	"github.com/farhapartex/coyote/core/model"
)

type widgetNarrow struct {
	ID    string `gorm:"primaryKey;size:64"`
	Label string `gorm:"not null;size:40"`
}

func (widgetNarrow) TableName() string { return "widgets" }

type widgetRetyped struct {
	ID    string  `gorm:"primaryKey;size:64"`
	Label float64 `gorm:"not null"`
}

func (widgetRetyped) TableName() string { return "widgets" }

func snapshotOf(t *testing.T, entity any) migrate.Snapshot {
	t.Helper()
	handle := newTestDB(t)
	return migrate.SnapshotOf([]*model.Schema{schemaFor(t, handle, entity)})
}

func TestAChangedSizeIsDetected(t *testing.T) {
	before := snapshotOf(t, widget{})
	after := snapshotOf(t, widgetNarrow{})

	change := migrate.Diff(before, after)
	if change.Empty() {
		t.Fatal("a changed column size must produce a migration, not silence")
	}

	alter, ok := change.Ops[0].(migrate.AlterColumn)
	if !ok {
		t.Fatalf("op = %T, want AlterColumn", change.Ops[0])
	}
	if alter.Table != "widgets" || alter.To.Name != "label" {
		t.Errorf("alter = %+v", alter)
	}
	if alter.From.Size != 120 || alter.To.Size != 40 {
		t.Errorf("sizes = %d -> %d, want 120 -> 40", alter.From.Size, alter.To.Size)
	}
	if len(alter.Columns) == 0 {
		t.Error("the whole table definition is needed to rebuild on sqlite")
	}

	if !strings.Contains(strings.Join(change.Warnings, " "), "narrows") {
		t.Errorf("narrowing should warn, got %v", change.Warnings)
	}
}

func TestAChangedTypeIsDetectedAndWarned(t *testing.T) {
	change := migrate.Diff(snapshotOf(t, widget{}), snapshotOf(t, widgetRetyped{}))

	if change.Empty() {
		t.Fatal("a changed column type must produce a migration")
	}
	joined := strings.Join(change.Warnings, " ")
	if !strings.Contains(joined, "changes type") {
		t.Errorf("a risky type change should warn, got %v", change.Warnings)
	}
}

func TestAnUnchangedModelStillProducesNothing(t *testing.T) {
	change := migrate.Diff(snapshotOf(t, widget{}), snapshotOf(t, widget{}))
	if !change.Empty() {
		t.Errorf("an unchanged model should produce no ops, got %d", len(change.Ops))
	}
}

func TestAlterColumnInvertsItself(t *testing.T) {
	change := migrate.Diff(snapshotOf(t, widget{}), snapshotOf(t, widgetNarrow{}))
	alter := change.Ops[0].(migrate.AlterColumn)

	inverse, ok := alter.Inverse()
	if !ok {
		t.Fatal("AlterColumn should be reversible")
	}
	back := inverse.(migrate.AlterColumn)
	if back.From.Size != 40 || back.To.Size != 120 {
		t.Errorf("inverse sizes = %d -> %d, want 40 -> 120", back.From.Size, back.To.Size)
	}
	for _, column := range back.Columns {
		if column.Name == "label" && column.Size != 120 {
			t.Errorf("the inverse table definition should carry the original size, got %d", column.Size)
		}
	}
}

func TestAlterColumnSQLPerDialect(t *testing.T) {
	from := dialect.Column{Name: "label", Type: "VARCHAR(120)"}
	to := dialect.Column{Name: "label", Type: "VARCHAR(40)", NotNull: true, Default: "'x'"}

	postgres := dialect.Postgres{}.AlterColumn("widgets", from, to)
	joined := strings.Join(postgres, " | ")
	for _, want := range []string{"TYPE VARCHAR(40)", "SET NOT NULL", "SET DEFAULT"} {
		if !strings.Contains(joined, want) {
			t.Errorf("postgres missing %q: %s", want, joined)
		}
	}

	mysql := dialect.MySQL{}.AlterColumn("widgets", from, to)
	if len(mysql) != 1 || !strings.Contains(mysql[0], "MODIFY COLUMN") {
		t.Errorf("mysql should restate the column in one statement: %v", mysql)
	}
	if !strings.Contains(mysql[0], "NOT NULL") {
		t.Errorf("mysql must restate nullability: %v", mysql)
	}

	if statements := (dialect.SQLite{}).AlterColumn("widgets", from, to); statements != nil {
		t.Errorf("sqlite has no ALTER COLUMN; it should report nothing and rebuild instead: %v", statements)
	}
}

func TestSQLiteRebuildPreservesData(t *testing.T) {
	handle := newTestDB(t)
	if err := handle.Exec(`CREATE TABLE widgets (id text primary key, label varchar(120) not null)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := handle.Exec(`INSERT INTO widgets VALUES ('a', 'keep me')`).Error; err != nil {
		t.Fatal(err)
	}

	change := migrate.Diff(snapshotOf(t, widget{}), snapshotOf(t, widgetNarrow{}))
	runner := migrate.NewRunner(handle, "sqlite", nil)
	if err := runner.Prepare(t.Context()); err != nil {
		t.Fatal(err)
	}

	m := migrate.Migration{ID: "0002_narrow", Up: change.Ops}
	if err := runner.Apply(t.Context(), m); err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}

	rows := []map[string]any{}
	if err := handle.Raw("SELECT id, label FROM widgets").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("the rebuild lost rows: %+v", rows)
	}
	if rows[0]["label"] != "keep me" {
		t.Errorf("value = %v, want the original", rows[0]["label"])
	}

	applied, err := runner.Applied(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0].ID != "0002_narrow" {
		t.Errorf("the rebuild should be recorded once: %+v", applied)
	}

	if handle.Migrator().HasTable("widgets__rebuilt") {
		t.Error("the shadow table should be gone")
	}
}

func TestAutoIncrementReachesEachDialect(t *testing.T) {
	for name, want := range map[string]string{
		"postgres": "BIGSERIAL",
		"mysql":    "AUTO_INCREMENT",
	} {
		var d dialect.Dialect = dialect.Postgres{}
		if name == "mysql" {
			d = dialect.MySQL{}
		}
		sql := d.CreateTable("counters", []dialect.Column{{
			Name: "id", Type: dialect.TypeFor(d, model.KindInt, 0, true), PrimaryKey: true, AutoIncrement: true,
		}})
		if !strings.Contains(sql, want) {
			t.Errorf("%s: an autoincrement key should emit %q:\n%s", name, want, sql)
		}
	}

	sql := dialect.SQLite{}.CreateTable("counters", []dialect.Column{{
		Name: "id", Type: dialect.TypeFor(dialect.SQLite{}, model.KindInt, 0, true), PrimaryKey: true, AutoIncrement: true,
	}})
	if !strings.Contains(sql, "INTEGER") || !strings.Contains(sql, "PRIMARY KEY") {
		t.Errorf("sqlite should use the rowid form:\n%s", sql)
	}
}
