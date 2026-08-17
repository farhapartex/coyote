package store

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

func (s *store) WithTx(tx *gorm.DB) model.Store {
	if tx == nil {
		return s
	}
	return &store{handle: tx}
}
