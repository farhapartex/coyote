package app

import (
	"fmt"
	"sync"

	"github.com/farhapartex/coyote/core/db"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
	"gorm.io/gorm"
)

type connection struct {
	once   sync.Once
	handle *gorm.DB
	err    error
}

func (a *App) DBByAlias(alias string) (*gorm.DB, error) {
	if alias == "" || alias == a.Settings.Database().Alias {
		return a.DB()
	}

	cfg, found := a.Settings.DatabaseByAlias(alias)
	if !found {
		return nil, fmt.Errorf("coyote/app: no database configured with alias %q", alias)
	}

	stored, _ := a.extras.LoadOrStore(alias, &connection{})
	entry := stored.(*connection)
	entry.once.Do(func() {
		entry.handle, entry.err = db.Open(cfg, db.Options{Logger: a.Logger, Debug: a.Settings.Debug})
	})
	return entry.handle, entry.err
}

func (a *App) DBFor(entity any) (*gorm.DB, error) {
	return a.DBByAlias(a.models.AliasOf(entity))
}

func (a *App) StoreForAlias(alias string) (model.Store, error) {
	handle, err := a.DBByAlias(alias)
	if err != nil {
		return nil, err
	}
	return store.New(handle), nil
}

func (a *App) StoreFor(entity any) (model.Store, error) {
	return a.StoreForAlias(a.models.AliasOf(entity))
}

func (a *App) closeExtras() {
	a.extras.Range(func(_, value any) bool {
		if entry, ok := value.(*connection); ok && entry.handle != nil {
			_ = db.Close(entry.handle)
		}
		return true
	})
}
