package app

import (
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/repo"
)

func (a *App) Store() (model.Store, error) {
	handle, err := a.DB()
	if err != nil {
		return nil, err
	}
	a.storeOnce.Do(func() { a.store = repo.New(handle) })
	return a.store, nil
}

func (a *App) Describe(entity any) (*model.Schema, error) {
	handle, err := a.DB()
	if err != nil {
		return nil, err
	}
	return model.Describe(handle, entity)
}
