package model

import (
	"fmt"

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
		Label:  Titleise(parsed.Name),
		Plural: Humanise(parsed.Table),
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
			Name:       field.Name,
			Column:     field.DBName,
			Label:      Humanise(field.DBName),
			Kind:       kind,
			Size:       field.Size,
			Nullable:   nullable || !field.NotNull,
			Required:   field.NotNull && !field.HasDefaultValue,
			PrimaryKey: field.PrimaryKey,
			Generated:  field.AutoIncrement || isTimestamp(field.DBName),
			Sensitive:  isSensitive(field.DBName),
		}
		if described.PrimaryKey {
			described.Required = false
			out.Key = described
		}
		out.Fields = append(out.Fields, described)
	}

	if out.Key.Column == "" {
		return nil, fmt.Errorf("coyote/model: %s has no primary key", out.Table)
	}
	return out, nil
}

func isTimestamp(column string) bool {
	return column == "created_at" || column == "updated_at" || column == "deleted_at"
}
