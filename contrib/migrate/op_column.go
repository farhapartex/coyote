package migrate

import (
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
)

type AddColumn struct {
	Table  string
	Column Column
}

func (o AddColumn) Describe() string {
	return fmt.Sprintf("add column %s.%s", o.Table, o.Column.Name)
}

func (o AddColumn) Statements(d dialect.Dialect) []string {
	return []string{d.AddColumn(o.Table, dialectColumns(d, []Column{o.Column})[0])}
}

func (o AddColumn) Source() string {
	return fmt.Sprintf("migrate.AddColumn{Table: %q, Column: %s}", o.Table, columnSource(o.Column))
}

type DropColumn struct {
	Table  string
	Column string
}

func (o DropColumn) Describe() string {
	return fmt.Sprintf("drop column %s.%s", o.Table, o.Column)
}

func (o DropColumn) Statements(d dialect.Dialect) []string {
	return []string{d.DropColumn(o.Table, o.Column)}
}

func (o DropColumn) Source() string {
	return fmt.Sprintf("migrate.DropColumn{Table: %q, Column: %q}", o.Table, o.Column)
}

type RenameColumn struct {
	Table string
	From  string
	To    string
}

func (o RenameColumn) Describe() string {
	return fmt.Sprintf("rename column %s.%s to %s", o.Table, o.From, o.To)
}

func (o RenameColumn) Statements(d dialect.Dialect) []string {
	return []string{d.RenameColumn(o.Table, o.From, o.To)}
}

func (o RenameColumn) Source() string {
	return fmt.Sprintf("migrate.RenameColumn{Table: %q, From: %q, To: %q}", o.Table, o.From, o.To)
}
