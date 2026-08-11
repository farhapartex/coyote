package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

func Open(cfg settings.Database, opts Options) (*gorm.DB, error) {
	if err := ensureDirectory(cfg); err != nil {
		return nil, err
	}
	dialector, err := Dialector(cfg)
	if err != nil {
		return nil, err
	}

	handle, err := gorm.Open(dialector, gormConfig(opts))
	if err != nil {
		return nil, fmt.Errorf("coyote/db: opening %s: %w", cfg.Alias, err)
	}
	pool, err := handle.DB()
	if err != nil {
		return nil, fmt.Errorf("coyote/db: pool for %s: %w", cfg.Alias, err)
	}
	applyPool(pool, cfg)
	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("coyote/db: connecting to %s: %w", cfg.Alias, err)
	}
	if err := applyPragmas(handle, cfg); err != nil {
		return nil, err
	}
	return handle, nil
}

func Close(handle *gorm.DB) error {
	if handle == nil {
		return nil
	}
	pool, err := handle.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}

func Pool(handle *gorm.DB) (*sql.DB, error) {
	if handle == nil {
		return nil, fmt.Errorf("coyote/db: no connection")
	}
	return handle.DB()
}

func ensureDirectory(cfg settings.Database) error {
	if !cfg.IsSQLite() || cfg.Name == ":memory:" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Name), 0o755); err != nil {
		return fmt.Errorf("coyote/db: creating directory for %s: %w", cfg.Name, err)
	}
	return nil
}
