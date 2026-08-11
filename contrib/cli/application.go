package cli

import (
	"log/slog"

	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

type Application interface {
	Serve() error
	Config() settings.Settings
	Models() []model.Model
	DB() (*gorm.DB, error)
	Log() *slog.Logger
}
