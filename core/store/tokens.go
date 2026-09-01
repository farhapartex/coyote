package store

import (
	"context"
	"errors"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tokenStore struct {
	resolve Resolver
}

func Tokens(handle *gorm.DB) auth.TokenStore {
	return LazyTokens(func() (*gorm.DB, error) { return handle, nil })
}

func LazyTokens(resolve Resolver) auth.TokenStore {
	return &tokenStore{resolve: resolve}
}

func (s *tokenStore) handle(ctx context.Context) (*gorm.DB, error) {
	if s.resolve == nil {
		return nil, errors.New("coyote/store: no database resolver configured")
	}
	handle, err := s.resolve()
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, errors.New("coyote/store: no database connection")
	}
	return handle, nil
}

func (s *tokenStore) Save(ctx context.Context, token auth.ResetToken) error {
	handle, err := s.handle(ctx)
	if err != nil {
		return err
	}
	return handle.Clauses(clause.OnConflict{UpdateAll: true}).Create(&token).Error
}

func (s *tokenStore) ByDigest(ctx context.Context, digest string) (auth.ResetToken, error) {
	handle, err := s.handle(ctx)
	if err != nil {
		return auth.ResetToken{}, err
	}
	rows := []auth.ResetToken{}
	if err := handle.Where("digest = ?", digest).Limit(1).Find(&rows).Error; err != nil {
		return auth.ResetToken{}, err
	}
	if len(rows) == 0 {
		return auth.ResetToken{}, ErrNotFound
	}
	return rows[0], nil
}

func (s *tokenStore) MarkUsed(ctx context.Context, digest string, at time.Time) error {
	handle, err := s.handle(ctx)
	if err != nil {
		return err
	}
	return handle.Model(&auth.ResetToken{}).
		Where("digest = ?", digest).
		Update("used_at", at).Error
}

func (s *tokenStore) DeleteForUser(ctx context.Context, userID string) error {
	handle, err := s.handle(ctx)
	if err != nil {
		return err
	}
	return handle.Where("user_id = ?", userID).Delete(&auth.ResetToken{}).Error
}

func (s *tokenStore) Sweep(ctx context.Context, before time.Time) (int, error) {
	handle, err := s.handle(ctx)
	if err != nil {
		return 0, err
	}
	result := handle.Where("expires_at < ?", before).Delete(&auth.ResetToken{})
	return int(result.RowsAffected), result.Error
}
