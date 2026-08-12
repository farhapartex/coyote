package migrate

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Sync(handle *gorm.DB, models []model.Model) error {
	if len(models) == 0 {
		return nil
	}
	entities := make([]any, 0, len(models))
	for _, m := range models {
		entities = append(entities, m.Entity)
	}
	quiet := handle.Session(&gorm.Session{Logger: logger.Discard})
	if err := quiet.AutoMigrate(entities...); err != nil {
		return fmt.Errorf("coyote/migrate: sync: %w", err)
	}
	return nil
}
