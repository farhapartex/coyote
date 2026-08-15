package app

import (
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/session"
)

func defaultModels(s Settings) *model.Registry {
	registry := model.NewRegistry(model.Of(auth.User{}))
	if s.Sessions.Backend.Persistent() {
		registry.Add(model.Of(session.Record{}))
	}
	return registry
}

func (a *App) RegisterModel(models ...model.Model) {
	a.models.Add(models...)
}

func (a *App) Registry() *model.Registry { return a.models }

func (a *App) Models() []model.Model { return a.models.All() }
