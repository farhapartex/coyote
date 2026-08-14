package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/farhapartex/coyote/lib/text"
	"gorm.io/gorm"
)

func Describe(handle *gorm.DB, entity any) (*Schema, error) {
	statement := &gorm.Statement{DB: handle}
	if err := statement.Parse(entity); err != nil {
		return nil, fmt.Errorf("coyote/model: parsing %T: %w", entity, err)
	}
	parsed := statement.Schema

	out := &Schema{
		Table:  parsed.Table,
		Slug:   parsed.Table,
		Label:  text.Titleise(parsed.Name),
		Plural: text.Humanise(parsed.Table),
	}

	for _, column := range parsed.DBNames {
		field := parsed.LookUpField(column)
		if field == nil {
			continue
		}
		kind, nullable := KindOf(field.FieldType)
		if kind == KindString && field.Size > 500 {
			kind = KindText
		}
		described := Field{
			Name:          field.Name,
			Column:        field.DBName,
			Label:         text.Humanise(labelFor(field.DBName, kind)),
			Kind:          kind,
			Size:          field.Size,
			Nullable:      nullable || !field.NotNull,
			Required:      field.NotNull && !field.HasDefaultValue,
			PrimaryKey:    field.PrimaryKey,
			AutoIncrement: field.AutoIncrement,
			Generated:     field.AutoIncrement || isTimestamp(field.DBName),
			Sensitive:     isSensitive(field.DBName),
			Default:       field.DefaultValue,
		}
		if described.PrimaryKey {
			described.Required = false
			out.Key = described
		}
		out.Fields = append(out.Fields, described)
	}

	for _, index := range parsed.ParseIndexes() {
		columns := make([]string, 0, len(index.Fields))
		for _, field := range index.Fields {
			columns = append(columns, field.DBName)
		}
		if len(columns) == 0 {
			continue
		}
		out.Indexes = append(out.Indexes, Index{
			Name:    index.Name,
			Columns: columns,
			Unique:  strings.EqualFold(index.Class, "UNIQUE"),
		})
	}
	sort.Slice(out.Indexes, func(i, j int) bool { return out.Indexes[i].Name < out.Indexes[j].Name })

	if out.Key.Column == "" {
		return nil, fmt.Errorf("coyote/model: %s has no primary key", out.Table)
	}
	return out, nil
}

func labelFor(column string, kind Kind) string {
	if kind == KindTime {
		return strings.TrimSuffix(column, "_at")
	}
	return column
}

func isTimestamp(column string) bool {
	return column == "created_at" || column == "updated_at" || column == "deleted_at"
}
