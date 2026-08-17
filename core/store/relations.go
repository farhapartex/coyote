package store

import (
	"context"
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm/clause"
)

const LabelSuffix = "__label"

func (s *store) resolve(ctx context.Context, schema *model.Schema, query model.Query, records []model.Record) error {
	if len(query.With) == 0 || len(records) == 0 {
		return nil
	}

	for _, name := range query.With {
		relation, found := schema.Relation(name)
		if !found || relation.LabelColumn == "" {
			continue
		}
		if err := s.attachLabels(ctx, relation, records); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) attachLabels(ctx context.Context, relation model.Relation, records []model.Record) error {
	keys := make([]any, 0, len(records))
	seen := map[any]bool{}
	for _, record := range records {
		value := record.Get(relation.Column)
		if value == nil || seen[value] {
			continue
		}
		seen[value] = true
		keys = append(keys, value)
	}
	if len(keys) == 0 {
		return nil
	}

	rows := []map[string]any{}
	err := s.handle.WithContext(ctx).
		Table(relation.Target).
		Select([]string{relation.TargetKey, relation.LabelColumn}).
		Clauses(clause.IN{Column: clause.Column{Name: relation.TargetKey}, Values: keys}).
		Find(&rows).Error
	if err != nil {
		return fmt.Errorf("coyote/repo: resolving %s: %w", relation.Target, err)
	}

	labels := make(map[string]any, len(rows))
	for _, row := range rows {
		labels[fmt.Sprint(row[relation.TargetKey])] = row[relation.LabelColumn]
	}
	for _, record := range records {
		if value := record.Get(relation.Column); value != nil {
			if label, found := labels[fmt.Sprint(value)]; found {
				record[relation.Column+LabelSuffix] = label
			}
		}
	}
	return nil
}

func (s *store) Options(ctx context.Context, relation model.Relation, limit int) ([]model.Record, error) {
	if relation.LabelColumn == "" {
		return nil, nil
	}
	session := s.handle.WithContext(ctx).
		Table(relation.Target).
		Select([]string{relation.TargetKey, relation.LabelColumn}).
		Order(relation.LabelColumn)
	if limit > 0 {
		session = session.Limit(limit)
	}

	rows := []map[string]any{}
	if err := session.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("coyote/repo: listing %s: %w", relation.Target, err)
	}
	out := make([]model.Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.Record(row))
	}
	return out, nil
}
