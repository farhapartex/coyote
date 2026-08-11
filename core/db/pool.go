package db

import (
	"database/sql"

	"github.com/farhapartex/coyote/core/settings"
)

func applyPool(handle *sql.DB, cfg settings.Database) {
	if cfg.MaxOpenConns > 0 {
		handle.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		handle.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		handle.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		handle.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}
