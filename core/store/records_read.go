package store

import (
	"context"

	"errors"
	"fmt"

	"github.com/farhapartex/coyote/core/model"
)

var ErrNotFound = errors.New("coyote/repo: record not found")

func (s *store) List(ctx context.Context, schema *model.Schema, query model.Query) (model.Page, error) {
	var page model.Page

	counter := s.handle.WithContext(ctx).Table(schema.Table)
	if err := counter.Count(&page.Total).Error; err != nil {
		return page, fmt.Errorf("coyote/repo: counting %s: %w", schema.Table, err)
	}

	rows := []map[string]any{}
	session := s.handle.WithContext(ctx).Table(schema.Table)
	if order := query.Order; order != "" {
		session = session.Order(order)
	} else {
		session = session.Order(schema.Key.Column)
	}
	if query.Limit > 0 {
		session = session.Limit(query.Limit)
	}
	if query.Offset > 0 {
		session = session.Offset(query.Offset)
	}
	if err := session.Find(&rows).Error; err != nil {
		return page, fmt.Errorf("coyote/repo: listing %s: %w", schema.Table, err)
	}

	page.Records = make([]model.Record, 0, len(rows))
	for _, row := range rows {
		page.Records = append(page.Records, model.Record(row))
	}
	return page, nil
}

func (s *store) Find(ctx context.Context, schema *model.Schema, id string) (model.Record, error) {
	rows := []map[string]any{}
	err := s.handle.WithContext(ctx).
		Table(schema.Table).
		Where(schema.Key.Column+" = ?", id).
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("coyote/repo: reading %s: %w", schema.Table, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s %s", ErrNotFound, schema.Table, id)
	}
	return model.Record(rows[0]), nil
}
