package repo

import (
	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
)

type store struct {
	handle *gorm.DB
}

func New(handle *gorm.DB) model.Store {
	return &store{handle: handle}
}
