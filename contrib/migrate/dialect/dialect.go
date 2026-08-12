package dialect

import "github.com/farhapartex/coyote/core/settings"

type Column struct {
	Name          string
	Type          string
	NotNull       bool
	PrimaryKey    bool
	AutoIncrement bool
	Default       string
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
	AddColumn(table string, column Column) string
	DropColumn(table, column string) string
	RenameColumn(table, from, to string) string
	CreateIndex(index Index) string
	DropIndex(table, name string) string
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
