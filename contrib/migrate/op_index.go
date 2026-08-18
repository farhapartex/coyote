package migrate

import (
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
)

type CreateIndex struct {
	Table   string
	Name    string
	Columns []string
	Unique  bool
}

func (o CreateIndex) Describe() string {
	kind := "index"
	if o.Unique {
		kind = "unique index"
	}
	return fmt.Sprintf("create %s %s on %s", kind, o.Name, o.Table)
}

func (o CreateIndex) Statements(d dialect.Dialect) []string {
	return []string{d.CreateIndex(dialect.Index{
		Name: o.Name, Table: o.Table, Columns: o.Columns, Unique: o.Unique,
	})}
}

func (o CreateIndex) Source() string {
	return fmt.Sprintf("migrate.CreateIndex{Table: %q, Name: %q, Columns: %s, Unique: %t}",
		o.Table, o.Name, stringsSource(o.Columns), o.Unique)
}

type DropIndex struct {
	Table   string
	Name    string
	Columns []string
	Unique  bool
}

func (o DropIndex) Describe() string {
	return fmt.Sprintf("drop index %s on %s", o.Name, o.Table)
}

func (o DropIndex) Statements(d dialect.Dialect) []string {
	return []string{d.DropIndex(o.Table, o.Name)}
}

func (o DropIndex) Source() string {
	return fmt.Sprintf("migrate.DropIndex{Table: %q, Name: %q}", o.Table, o.Name)
}
