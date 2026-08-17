package store

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrBadFilter = fmt.Errorf("coyote/repo: unusable filter")

func (s *store) session(schema *model.Schema, query model.Query) (*gorm.DB, error) {
	session := s.handle.Table(schema.Table)

	for _, filter := range query.Filters {
		expression, err := condition(schema, filter)
		if err != nil {
			return nil, err
		}
		session = session.Where(expression)
	}
	return session, nil
}

func condition(schema *model.Schema, filter model.Filter) (clause.Expression, error) {
	if !schema.HasColumn(filter.Column) {
		return nil, fmt.Errorf("%w: %s has no column %q", ErrBadFilter, schema.Table, filter.Column)
	}
	column := clause.Column{Name: filter.Column}

	switch filter.Op {
	case model.Eq, "":
		return clause.Eq{Column: column, Value: filter.Value}, nil
	case model.Ne:
		return clause.Neq{Column: column, Value: filter.Value}, nil
	case model.Lt:
		return clause.Lt{Column: column, Value: filter.Value}, nil
	case model.Lte:
		return clause.Lte{Column: column, Value: filter.Value}, nil
	case model.Gt:
		return clause.Gt{Column: column, Value: filter.Value}, nil
	case model.Gte:
		return clause.Gte{Column: column, Value: filter.Value}, nil
	case model.Like:
		return clause.Like{Column: column, Value: filter.Value}, nil
	case model.In:
		values, ok := filter.Value.([]any)
		if !ok {
			return nil, fmt.Errorf("%w: %s with In needs a []any value", ErrBadFilter, filter.Column)
		}
		if len(values) == 0 {
			return clause.Expr{SQL: "1 = 0"}, nil
		}
		return clause.IN{Column: column, Values: values}, nil
	case model.Null:
		return clause.Eq{Column: column, Value: nil}, nil
	case model.NotNull:
		return clause.Neq{Column: column, Value: nil}, nil
	}
	return nil, fmt.Errorf("%w: unknown operator %q", ErrBadFilter, filter.Op)
}

func ordering(schema *model.Schema, query model.Query) clause.OrderBy {
	if column, direction, ok := schema.SortColumn(query.Sort); ok {
		return clause.OrderBy{Columns: []clause.OrderByColumn{{
			Column: clause.Column{Name: column},
			Desc:   direction == "desc",
		}}}
	}
	if query.Order != "" {
		return clause.OrderBy{Expression: clause.Expr{SQL: query.Order}}
	}
	return clause.OrderBy{Columns: []clause.OrderByColumn{{
		Column: clause.Column{Name: schema.Key.Column},
	}}}
}

func selection(schema *model.Schema, query model.Query) []string {
	if len(query.Select) == 0 {
		return nil
	}
	columns := make([]string, 0, len(query.Select)+1)
	seen := map[string]bool{}
	for _, column := range append([]string{schema.Key.Column}, query.Select...) {
		if seen[column] || !schema.HasColumn(column) {
			continue
		}
		seen[column] = true
		columns = append(columns, column)
	}
	return columns
}
