package db

import (
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/core/settings"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var ErrUnsupportedEngine = errors.New("coyote/db: unsupported engine")

func Dialector(cfg settings.Database) (gorm.Dialector, error) {
	switch cfg.Engine {
	case settings.SQLite:
		return sqlite.Open(cfg.DSN()), nil
	default:
		return nil, fmt.Errorf("%w: %q; only sqlite has a bundled driver so far", ErrUnsupportedEngine, cfg.Engine)
	}
}
