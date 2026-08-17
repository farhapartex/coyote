package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("coyote/repo: record not found")

func (s *store) List(ctx context.Context, schema *model.Schema, query model.Query) (model.Page, error) {
	var page model.Page

	counted, err := s.Count(ctx, schema, query)
	if err != nil {
		return page, err
	}
	page.Total = counted

	session, err := s.session(schema, query)
	if err != nil {
		return page, err
	}
	session = session.WithContext(ctx).Clauses(ordering(schema, query))
	if columns := selection(schema, query); columns != nil {
		session = session.Select(columns)
	}
	if query.Limit > 0 {
		session = session.Limit(query.Limit)
	}
	if query.Offset > 0 {
		session = session.Offset(query.Offset)
	}

	rows := []map[string]any{}
	if err := session.Find(&rows).Error; err != nil {
		return page, fmt.Errorf("coyote/repo: listing %s: %w", schema.Table, err)
	}

	page.Records = make([]model.Record, 0, len(rows))
	for _, row := range rows {
		page.Records = append(page.Records, model.Record(row))
	}
	if err := s.resolve(ctx, schema, query, page.Records); err != nil {
		return page, err
	}
	return page, nil
}

func (s *store) Count(ctx context.Context, schema *model.Schema, query model.Query) (int64, error) {
	session, err := s.session(schema, query)
	if err != nil {
		return 0, err
	}
	var total int64
	if err := session.WithContext(ctx).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("coyote/repo: counting %s: %w", schema.Table, err)
	}
	return total, nil
}

func (s *store) Exists(ctx context.Context, schema *model.Schema, query model.Query) (bool, error) {
	query.Limit, query.Offset = 1, 0
	total, err := s.Count(ctx, schema, query)
	return total > 0, err
}

func (s *store) First(ctx context.Context, schema *model.Schema, query model.Query) (model.Record, error) {
	query.Limit = 1
	page, err := s.List(ctx, schema, query)
	if err != nil {
		return nil, err
	}
	if len(page.Records) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, schema.Table)
	}
	return page.Records[0], nil
}

func (s *store) Find(ctx context.Context, schema *model.Schema, id string) (model.Record, error) {
	rows := []map[string]any{}
	err := s.handle.WithContext(ctx).
		Table(schema.Table).
		Clauses(clause.Eq{Column: clause.Column{Name: schema.Key.Column}, Value: id}).
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
