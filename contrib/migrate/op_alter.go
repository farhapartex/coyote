package migrate

import (
	"context"
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
	"gorm.io/gorm"
)

type AlterColumn struct {
	Table   string
	From    Column
	To      Column
	Columns []Column
	Indexes []Index
}

func (o AlterColumn) Describe() string {
	return fmt.Sprintf("alter column %s.%s", o.Table, o.To.Name)
}

func (o AlterColumn) Statements(d dialect.Dialect) []string {
	if !o.Rebuilds(d) {
		return d.AlterColumn(o.Table, o.dialectColumn(d, o.From), o.dialectColumn(d, o.To))
	}
	statements, err := o.rebuildStatements(d)
	if err != nil {
		return []string{"-- " + o.Describe() + ": " + err.Error()}
	}
	return statements
}

func (o AlterColumn) dialectColumn(d dialect.Dialect, column Column) dialect.Column {
	return dialectColumns(d, []Column{column})[0]
}

func (o AlterColumn) Rebuilds(d dialect.Dialect) bool {
	rebuilder, ok := d.(dialect.Rebuilder)
	return ok && rebuilder.NeedsRebuild()
}

func (o AlterColumn) Exec(ctx context.Context, handle *gorm.DB) error {
	return fmt.Errorf("coyote/migrate: %s needs a dialect-aware runner", o.Describe())
}

func (o AlterColumn) Source() string {
	return fmt.Sprintf("migrate.AlterColumn{Table: %q, From: %s, To: %s, Columns: %s, Indexes: %s}",
		o.Table, columnLiteral(o.From), columnLiteral(o.To), columnsSource(o.Columns), indexesSource(o.Indexes))
}

func (o AlterColumn) Inverse() (Op, bool) {
	swapped := AlterColumn{Table: o.Table, From: o.To, To: o.From, Indexes: o.Indexes}
	for _, column := range o.Columns {
		if column.Name == o.To.Name {
			column = o.From
		}
		swapped.Columns = append(swapped.Columns, column)
	}
	return swapped, true
}

func (o AlterColumn) rebuildStatements(d dialect.Dialect) ([]string, error) {
	rebuilder, ok := d.(interface {
		Rebuild(table string, columns []dialect.Column, indexes []dialect.Index, copied []string) []string
	})
	if !ok {
		return nil, fmt.Errorf("coyote/migrate: %s cannot rebuild tables", d.Name())
	}
	if len(o.Columns) == 0 {
		return nil, fmt.Errorf("coyote/migrate: %s needs the full table definition to rebuild %s", o.Describe(), o.Table)
	}

	copied := make([]string, 0, len(o.Columns))
	for _, column := range o.Columns {
		copied = append(copied, column.Name)
	}
	return rebuilder.Rebuild(o.Table, dialectColumns(d, o.Columns), dialectIndexes(o.Indexes, o.Table), copied), nil
}

func dialectIndexes(indexes []Index, table string) []dialect.Index {
	out := make([]dialect.Index, 0, len(indexes))
	for _, index := range indexes {
		out = append(out, dialect.Index{
			Name: index.Name, Table: table, Columns: index.Columns, Unique: index.Unique,
		})
	}
	return out
}
