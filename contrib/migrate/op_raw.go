package migrate

import (
	"context"
	"fmt"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
	"gorm.io/gorm"
)

type RunSQL struct {
	Any      string
	SQLite   string
	Postgres string
	MySQL    string
	Note     string
}

func (o RunSQL) Describe() string {
	if o.Note != "" {
		return "run sql: " + o.Note
	}
	return "run sql"
}

func (o RunSQL) Statements(d dialect.Dialect) []string {
	statement := o.Any
	switch d.Name() {
	case "sqlite":
		if o.SQLite != "" {
			statement = o.SQLite
		}
	case "postgres":
		if o.Postgres != "" {
			statement = o.Postgres
		}
	case "mysql":
		if o.MySQL != "" {
			statement = o.MySQL
		}
	}
	if statement == "" {
		return nil
	}
	return []string{statement}
}

func (o RunSQL) Source() string {
	return fmt.Sprintf("migrate.RunSQL{Any: %q, Note: %q}", o.Any, o.Note)
}

type RunGo struct {
	Note string
	Func func(ctx context.Context, handle *gorm.DB) error
}

func (o RunGo) Describe() string {
	if o.Note != "" {
		return "run go: " + o.Note
	}
	return "run go"
}

func (o RunGo) Exec(ctx context.Context, handle *gorm.DB) error {
	if o.Func == nil {
		return nil
	}
	return o.Func(ctx, handle)
}
