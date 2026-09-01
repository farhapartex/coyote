package dialect

import (
	"fmt"
	"strings"

	"github.com/farhapartex/coyote/core/model"
)

type SQLite struct{}

func (SQLite) Name() string { return "sqlite" }

func (SQLite) Quote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (d SQLite) CreateTable(table string, columns []Column) string {
	return createTable(d, table, columns, true)
}

func (d SQLite) DropTable(table string) string {
	return "DROP TABLE " + d.Quote(table)
}

func (d SQLite) AddColumn(table string, column Column) []string {
	clause := columnClause(d, column)
	if !column.References.IsZero() {
		clause += fmt.Sprintf(" REFERENCES %s (%s)",
			d.Quote(column.References.Table), d.Quote(column.References.Column))
	}
	return []string{fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", d.Quote(table), clause)}
}

func (d SQLite) DropColumn(table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", d.Quote(table), d.Quote(column))
}

func (d SQLite) RenameColumn(table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", d.Quote(table), d.Quote(from), d.Quote(to))
}

func (d SQLite) CreateIndex(index Index) string { return createIndex(d, index) }

func (d SQLite) DropIndex(table, name string) string { return "DROP INDEX " + d.Quote(name) }

func SQLiteType(kind model.Kind, autoIncrement bool) string {
	if autoIncrement {
		return "INTEGER"
	}
	switch kind {
	case model.KindInt, model.KindBool:
		return "INTEGER"
	case model.KindFloat:
		return "REAL"
	case model.KindTime:
		return "DATETIME"
	case model.KindBytes:
		return "BLOB"
	default:
		return "TEXT"
	}
}

func (d SQLite) AlterColumn(table string, from, to Column) []string {
	return nil
}

func (d SQLite) Rebuild(table string, columns []Column, indexes []Index, copied []string) []string {
	shadow := table + "__rebuilt"

	statements := []string{
		createTableNamed(d, shadow, table, columns, true),
		fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
			d.Quote(shadow), joinColumns(d, copied), joinColumns(d, copied), d.Quote(table)),
		"DROP TABLE " + d.Quote(table),
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", d.Quote(shadow), d.Quote(table)),
	}
	for _, index := range indexes {
		statements = append(statements, createIndex(d, Index{
			Name: index.Name, Table: table, Columns: index.Columns, Unique: index.Unique,
		}))
	}
	return statements
}

func (SQLite) NeedsRebuild() bool { return true }
