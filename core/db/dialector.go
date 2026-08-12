package db

import (
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/core/settings"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrUnsupportedEngine = errors.New("coyote/db: unsupported engine")

func Dialector(cfg settings.Database) (gorm.Dialector, error) {
	switch cfg.Engine {
	case settings.SQLite:
		return sqlite.Open(cfg.DSN()), nil
	case settings.Postgres:
		return postgres.Open(cfg.DSN()), nil
	case settings.MySQL:
		return mysql.Open(cfg.DSN()), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedEngine, cfg.Engine)
	}
}
