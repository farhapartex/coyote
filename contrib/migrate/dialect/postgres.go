package dialect

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
)

type Postgres struct{}

func (Postgres) Name() string { return "postgres" }

func (Postgres) Quote(identifier string) string { return `"` + identifier + `"` }

func (d Postgres) CreateTable(table string, columns []Column) string {
	return createTable(d, table, columns, false)
}

func (d Postgres) DropTable(table string) string { return "DROP TABLE " + d.Quote(table) }

func (d Postgres) AddColumn(table string, column Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", d.Quote(table), columnClause(d, column))
}

func (d Postgres) DropColumn(table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", d.Quote(table), d.Quote(column))
}

func (d Postgres) RenameColumn(table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", d.Quote(table), d.Quote(from), d.Quote(to))
}

func (d Postgres) CreateIndex(index Index) string { return createIndex(d, index) }

func (d Postgres) DropIndex(table, name string) string { return "DROP INDEX " + d.Quote(name) }

func PostgresType(kind model.Kind, size int, autoIncrement bool) string {
	if autoIncrement {
		return "BIGSERIAL"
	}
	switch kind {
	case model.KindInt:
		return "BIGINT"
	case model.KindFloat:
		return "DOUBLE PRECISION"
	case model.KindBool:
		return "BOOLEAN"
	case model.KindTime:
		return "TIMESTAMPTZ"
	case model.KindBytes:
		return "BYTEA"
	case model.KindText:
		return "TEXT"
	default:
		if size > 0 {
			return fmt.Sprintf("VARCHAR(%d)", size)
		}
		return "TEXT"
	}
}
