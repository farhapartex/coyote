package db

import (
	"fmt"

	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

var sqlitePragmas = []string{
	"PRAGMA journal_mode = WAL",
	"PRAGMA busy_timeout = 5000",
	"PRAGMA foreign_keys = ON",
}

func applyPragmas(handle *gorm.DB, cfg settings.Database) error {
	if !cfg.IsSQLite() {
		return nil
	}
	for _, pragma := range sqlitePragmas {
		if err := handle.Exec(pragma).Error; err != nil {
			return fmt.Errorf("coyote/db: %s: %w", pragma, err)
		}
	}
	return nil
}
