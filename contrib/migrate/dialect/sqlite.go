package dialect

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
)

type SQLite struct{}

func (SQLite) Name() string { return "sqlite" }

func (SQLite) Quote(identifier string) string { return `"` + identifier + `"` }

func (d SQLite) CreateTable(table string, columns []Column) string {
	return createTable(d, table, columns, true)
}

func (d SQLite) DropTable(table string) string {
	return "DROP TABLE " + d.Quote(table)
}

func (d SQLite) AddColumn(table string, column Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", d.Quote(table), columnClause(d, column))
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
