package app

import (
	"database/sql"

	"github.com/farhapartex/coyote/core/db"
	"gorm.io/gorm"
)

func (a *App) DB() (*gorm.DB, error) {
	a.dbOnce.Do(func() {
		a.dbHandle, a.dbErr = db.Open(a.Settings.Database(), db.Options{
			Logger: a.Logger,
			Debug:  a.Settings.Debug,
		})
	})
	return a.dbHandle, a.dbErr
}

func (a *App) Pool() (*sql.DB, error) {
	handle, err := a.DB()
	if err != nil {
		return nil, err
	}
	return db.Pool(handle)
}

func (a *App) CloseDB() error {
	a.closeExtras()
	return db.Close(a.dbHandle)
}
