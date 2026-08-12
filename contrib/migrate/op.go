package migrate

import (
	"context"

	"github.com/farhapartex/coyote/contrib/migrate/dialect"
	"gorm.io/gorm"
)

type Op interface {
	Describe() string
}

type Statementer interface {
	Op
	Statements(d dialect.Dialect) []string
}

type Executor interface {
	Op
	Exec(ctx context.Context, handle *gorm.DB) error
}

type Sourcer interface {
	Op
	Source() string
}
