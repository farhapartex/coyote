package dialect

import "github.com/farhapartex/coyote/core/settings"

type Reference struct {
	Table  string
	Column string
}

func (r Reference) IsZero() bool { return r.Table == "" && r.Column == "" }

type Column struct {
	Name          string
	Type          string
	NotNull       bool
	PrimaryKey    bool
	AutoIncrement bool
	Default       string
	References    Reference
}

type Index struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
}

type Dialect interface {
	Name() string
	Quote(identifier string) string
	CreateTable(table string, columns []Column) string
	DropTable(table string) string
	AddColumn(table string, column Column) []string
	DropColumn(table, column string) string
	RenameColumn(table, from, to string) string
	CreateIndex(index Index) string
	DropIndex(table, name string) string
	AlterColumn(table string, from, to Column) []string
}

func For(engine settings.Engine) Dialect {
	switch engine {
	case settings.Postgres:
		return Postgres{}
	case settings.MySQL:
		return MySQL{}
	default:
		return SQLite{}
	}
}

type Rebuilder interface {
	NeedsRebuild() bool
}
