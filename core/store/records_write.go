package store

import (
	"context"
	"fmt"
	"time"

	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/lib/id"
)

func (s *store) Insert(ctx context.Context, schema *model.Schema, record model.Record) (string, error) {
	values := make(map[string]any, len(record)+2)
	for column, value := range record {
		values[column] = value
	}

	key := fmt.Sprintf("%v", values[schema.Key.Column])
	if schema.Key.Kind == model.KindString && (values[schema.Key.Column] == nil || key == "" || key == "<nil>") {
		generated, err := id.New()
		if err != nil {
			return "", err
		}
		values[schema.Key.Column] = generated
		key = generated
	}
	if schema.Key.Generated {
		delete(values, schema.Key.Column)
		key = ""
	}
	stampTimestamps(schema, values, true)

	if err := s.handle.WithContext(ctx).Table(schema.Table).Create(values).Error; err != nil {
		return "", fmt.Errorf("coyote/repo: creating %s: %w", schema.Table, err)
	}
	if key == "" {
		if assigned, ok := values[schema.Key.Column]; ok {
			key = fmt.Sprintf("%v", assigned)
		}
	}
	return key, nil
}

func (s *store) Update(ctx context.Context, schema *model.Schema, key string, record model.Record) error {
	values := make(map[string]any, len(record)+1)
	for column, value := range record {
		if column == schema.Key.Column {
			continue
		}
		values[column] = value
	}
	stampTimestamps(schema, values, false)

	result := s.handle.WithContext(ctx).
		Table(schema.Table).
		Where(schema.Key.Column+" = ?", key).
		Updates(values)
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: updating %s: %w", schema.Table, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s %s", ErrNotFound, schema.Table, key)
	}
	return nil
}

func (s *store) Delete(ctx context.Context, schema *model.Schema, key string) error {
	result := s.handle.WithContext(ctx).
		Table(schema.Table).
		Where(schema.Key.Column+" = ?", key).
		Delete(nil)
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: deleting %s: %w", schema.Table, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s %s", ErrNotFound, schema.Table, key)
	}
	return nil
}

func stampTimestamps(schema *model.Schema, values map[string]any, creating bool) {
	now := time.Now().UTC()
	if _, ok := schema.Field("updated_at"); ok {
		values["updated_at"] = now
	}
	if creating {
		if _, ok := schema.Field("created_at"); ok {
			values["created_at"] = now
		}
	}
}
