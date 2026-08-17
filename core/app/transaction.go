package app

import (
	"context"
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/upload"
	"gorm.io/gorm"
)

type Tx struct {
	handle  *gorm.DB
	store   model.Store
	uploads *upload.Service
	pending []*upload.Ref
}

func (t *Tx) DB() *gorm.DB { return t.handle }

func (t *Tx) Store() model.Store { return t.store }

func (t *Tx) Keep(refs ...*upload.Ref) {
	t.pending = append(t.pending, refs...)
}

func (a *App) Transaction(ctx context.Context, fn func(*Tx) error) error {
	handle, err := a.DB()
	if err != nil {
		return err
	}

	tx := &Tx{uploads: a.Uploads}
	err = handle.WithContext(ctx).Transaction(func(inner *gorm.DB) error {
		tx.handle = inner
		if records, err := a.Store(); err == nil {
			tx.store = records.WithTx(inner)
		}
		return fn(tx)
	})
	if err != nil {
		return err
	}

	if a.Uploads == nil || len(tx.pending) == 0 {
		return nil
	}
	if err := a.Uploads.Commit(ctx, tx.pending...); err != nil {
		return fmt.Errorf("coyote/app: the transaction committed but a file did not: %w", err)
	}
	return nil
}
