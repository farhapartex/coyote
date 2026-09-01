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

func (d Postgres) AddColumn(table string, column Column) []string {
	return addColumnWithConstraint(d, table, column)
}

func (d Postgres) DropColumn(table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", d.Quote(table), d.Quote(column))
}

func (d Postgres) RenameColumn(table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", d.Quote(table), d.Quote(from), d.Quote(to))
}

func (d Postgres) CreateIndex(index Index) string { return createIndex(d, index) }

func (d Postgres) DropIndex(table, name string) string { return "DROP INDEX " + d.Quote(name) }

func (d Postgres) AlterColumn(table string, from, to Column) []string {
	out := []string{}
	prefix := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s", d.Quote(table), d.Quote(to.Name))

	if from.Type != to.Type {
		out = append(out, fmt.Sprintf("%s TYPE %s USING %s::%s", prefix, to.Type, d.Quote(to.Name), to.Type))
	}
	if from.NotNull != to.NotNull {
		if to.NotNull {
			out = append(out, prefix+" SET NOT NULL")
		} else {
			out = append(out, prefix+" DROP NOT NULL")
		}
	}
	if from.Default != to.Default {
		if to.Default == "" {
			out = append(out, prefix+" DROP DEFAULT")
		} else {
			out = append(out, prefix+" SET DEFAULT "+to.Default)
		}
	}
	if from.References != to.References {
		if !from.References.IsZero() {
			out = append(out, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s",
				d.Quote(table), d.Quote(ForeignKeyName(table, to.Name))))
		}
		if !to.References.IsZero() {
			out = append(out, fmt.Sprintf("ALTER TABLE %s ADD %s", d.Quote(table), foreignKeyClause(d, table, to)))
		}
	}
	return out
}

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
