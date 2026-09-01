package migrate

import (
	"fmt"
	"strings"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
)

type CreateTable struct {
	Table   Table
	Comment string
}

func (o CreateTable) Describe() string { return "create table " + o.Table.Name }

func (o CreateTable) Statements(d dialect.Dialect) []string {
	out := []string{d.CreateTable(o.Table.Name, dialectColumns(d, o.Table.Columns))}
	for _, ix := range o.Table.Indexes {
		out = append(out, d.CreateIndex(dialect.Index{
			Name: ix.Name, Table: o.Table.Name, Columns: ix.Columns, Unique: ix.Unique,
		}))
	}
	return out
}

func (o CreateTable) Source() string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrate.CreateTable{Table: migrate.Table{\n\t\t\t\tName: %q,\n", o.Table.Name)
	b.WriteString("\t\t\t\tColumns: []migrate.Column{\n")
	for _, c := range o.Table.Columns {
		fmt.Fprintf(&b, "\t\t\t\t\t%s,\n", columnSource(c))
	}
	b.WriteString("\t\t\t\t},\n")
	if len(o.Table.Indexes) > 0 {
		b.WriteString("\t\t\t\tIndexes: []migrate.Index{\n")
		for _, ix := range o.Table.Indexes {
			fmt.Fprintf(&b, "\t\t\t\t\t{Name: %q, Columns: %s, Unique: %t},\n",
				ix.Name, stringsSource(ix.Columns), ix.Unique)
		}
		b.WriteString("\t\t\t\t},\n")
	}
	b.WriteString("\t\t\t}}")
	return b.String()
}

type DropTable struct {
	Name    string
	Restore Table `json:"-"`
}

func (o DropTable) Describe() string { return "drop table " + o.Name }

func (o DropTable) Statements(d dialect.Dialect) []string { return []string{d.DropTable(o.Name)} }

func (o DropTable) Source() string {
	return fmt.Sprintf("migrate.DropTable{Name: %q}", o.Name)
}

func dialectColumns(d dialect.Dialect, columns []Column) []dialect.Column {
	out := make([]dialect.Column, 0, len(columns))
	for _, c := range columns {
		out = append(out, dialect.Column{
			Name:          c.Name,
			Type:          dialect.TypeFor(d, c.Kind, c.Size, c.AutoIncrement),
			NotNull:       c.NotNull,
			PrimaryKey:    c.PrimaryKey,
			AutoIncrement: c.AutoIncrement,
			Default:       c.Default,
			References:    dialect.Reference{Table: c.References.Table, Column: c.References.Column},
		})
	}
	return out
}
