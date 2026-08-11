package app

import (
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
)

func defaultModels() *model.Registry {
	return model.NewRegistry(model.Of(auth.User{}))
}

func (a *App) RegisterModel(models ...model.Model) {
	a.models.Add(models...)
}

func (a *App) Registry() *model.Registry { return a.models }

func (a *App) Models() []model.Model { return a.models.All() }
