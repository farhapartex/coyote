package dialect

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
)

type MySQL struct{}

func (MySQL) Name() string { return "mysql" }

func (MySQL) Quote(identifier string) string { return "`" + identifier + "`" }

func (d MySQL) CreateTable(table string, columns []Column) string {
	return createTable(d, table, columns, false)
}

func (d MySQL) DropTable(table string) string { return "DROP TABLE " + d.Quote(table) }

func (d MySQL) AddColumn(table string, column Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", d.Quote(table), columnClause(d, column))
}

func (d MySQL) DropColumn(table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", d.Quote(table), d.Quote(column))
}

func (d MySQL) RenameColumn(table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", d.Quote(table), d.Quote(from), d.Quote(to))
}

func (d MySQL) CreateIndex(index Index) string { return createIndex(d, index) }

func (d MySQL) DropIndex(table, name string) string {
	return fmt.Sprintf("DROP INDEX %s ON %s", d.Quote(name), d.Quote(table))
}

func MySQLType(kind model.Kind, size int, autoIncrement bool) string {
	if autoIncrement {
		return "BIGINT AUTO_INCREMENT"
	}
	switch kind {
	case model.KindInt:
		return "BIGINT"
	case model.KindFloat:
		return "DOUBLE"
	case model.KindBool:
		return "BOOLEAN"
	case model.KindTime:
		return "DATETIME"
	case model.KindBytes:
		return "BLOB"
	case model.KindText:
		return "LONGTEXT"
	default:
		if size > 0 && size <= 4000 {
			return fmt.Sprintf("VARCHAR(%d)", size)
		}
		return "VARCHAR(191)"
	}
}
