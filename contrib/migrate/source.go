package migrate

import (
	"fmt"
	"strings"
)

func columnSource(c Column) string {
	parts := []string{fmt.Sprintf("Name: %q", c.Name), fmt.Sprintf("Kind: model.%s", kindConstant(c.Kind))}
	if c.Size > 0 {
		parts = append(parts, fmt.Sprintf("Size: %d", c.Size))
	}
	if c.NotNull {
		parts = append(parts, "NotNull: true")
	}
	if c.PrimaryKey {
		parts = append(parts, "PrimaryKey: true")
	}
	if c.AutoIncrement {
		parts = append(parts, "AutoIncrement: true")
	}
	if c.Default != "" {
		parts = append(parts, fmt.Sprintf("Default: %q", c.Default))
	}
	if !c.References.IsZero() {
		parts = append(parts, fmt.Sprintf("References: migrate.Reference{Table: %q, Column: %q}",
			c.References.Table, c.References.Column))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func columnLiteral(c Column) string {
	return "migrate.Column" + columnSource(c)
}

func stringsSource(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, fmt.Sprintf("%q", v))
	}
	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

func kindConstant(kind Kind) string {
	switch kind {
	case KindInt:
		return "KindInt"
	case KindFloat:
		return "KindFloat"
	case KindBool:
		return "KindBool"
	case KindTime:
		return "KindTime"
	case KindBytes:
		return "KindBytes"
	case KindText:
		return "KindText"
	default:
		return "KindString"
	}
}

func columnsSource(columns []Column) string {
	if len(columns) == 0 {
		return "nil"
	}
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, columnSource(column))
	}
	return "[]migrate.Column{" + strings.Join(parts, ", ") + "}"
}

func indexesSource(indexes []Index) string {
	if len(indexes) == 0 {
		return "nil"
	}
	parts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		unique := ""
		if index.Unique {
			unique = ", Unique: true"
		}
		parts = append(parts, fmt.Sprintf("{Name: %q, Columns: %s%s}", index.Name, stringsSource(index.Columns), unique))
	}
	return "[]migrate.Index{" + strings.Join(parts, ", ") + "}"
}
