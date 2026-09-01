package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/farhapartex/coyote/lib/text"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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

		options := ParseTag(field.Tag.Get(Tag))
		if looksLikeFile(field.FieldType) {
			options.IsFile = true
		}
		if options.IsFile {
			kind = KindFile
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
			Sensitive:     options.Sensitive || (!options.Public && isSensitive(field.DBName)),
			Default:       field.DefaultValue,
			UploadPath:    options.Path,
			Accept:        options.Accept,
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
	out.Relations = relationsOf(parsed)
	return out, nil
}

func relationsOf(parsed *schema.Schema) []Relation {
	if parsed.Relationships.Relations == nil {
		return nil
	}

	out := []Relation{}
	for _, relation := range parsed.Relationships.Relations {
		if relation.Type != schema.BelongsTo || len(relation.References) != 1 {
			continue
		}
		reference := relation.References[0]
		if reference.ForeignKey == nil || reference.PrimaryKey == nil {
			continue
		}
		out = append(out, Relation{
			Column:      reference.ForeignKey.DBName,
			Target:      relation.FieldSchema.Table,
			TargetKey:   reference.PrimaryKey.DBName,
			LabelColumn: labelColumnOf(relation.FieldSchema),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Column < out[j].Column })
	return out
}

func labelColumnOf(target *schema.Schema) string {
	for _, name := range []string{"name", "title", "label", "username", "email"} {
		if field := target.LookUpField(name); field != nil {
			return field.DBName
		}
	}
	for _, column := range target.DBNames {
		field := target.LookUpField(column)
		if field == nil || field.PrimaryKey {
			continue
		}
		if kind, _ := KindOf(field.FieldType); kind == KindString {
			return field.DBName
		}
	}
	if target.PrioritizedPrimaryField != nil {
		return target.PrioritizedPrimaryField.DBName
	}
	return ""
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
