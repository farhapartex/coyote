package cli

import (
	"context"
	"log/slog"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

type PermissionSyncer interface {
	SyncPermissions(ctx context.Context) (auth.SyncReport, error)
}

type Report struct {
	Created []string
	Stale   []string
}

type Application interface {
	Serve() error
	Config() settings.Settings
	Models() []model.Model
	DB() (*gorm.DB, error)
	AuthService() *auth.Service
	Log() *slog.Logger
}
