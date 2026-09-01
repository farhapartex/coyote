package model

import "strings"

const listColumnLimit = 4

type Field struct {
	Name          string
	Column        string
	Label         string
	Kind          Kind
	Size          int
	Required      bool
	Nullable      bool
	PrimaryKey    bool
	AutoIncrement bool
	Generated     bool
	Sensitive     bool
	Default       string
	UploadPath    string
	Accept        string
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

func (f Field) Editable() bool { return !f.Generated && !f.PrimaryKey }

type Relation struct {
	Column      string
	Target      string
	TargetKey   string
	LabelColumn string
}

type Schema struct {
	Table     string
	Slug      string
	Label     string
	Plural    string
	Fields    []Field
	Indexes   []Index
	Relations []Relation
	Key       Field
	listOnly  []string
	hidden    map[string]bool
}

func (s *Schema) Field(column string) (Field, bool) {
	for _, f := range s.Fields {
		if f.Column == column {
			return f, true
		}
	}
	return Field{}, false
}

func (s *Schema) Visible() []Field {
	out := make([]Field, 0, len(s.Fields))
	for _, f := range s.Fields {
		if !s.hidden[f.Column] {
			out = append(out, f)
		}
	}
	return out
}

func (s *Schema) FormFields() []Field {
	out := make([]Field, 0, len(s.Fields))
	for _, f := range s.Visible() {
		if f.Editable() {
			out = append(out, f)
		}
	}
	return out
}

func (s *Schema) DisplayFields(includeKey bool) []Field {
	out := make([]Field, 0, len(s.Fields)+1)
	if includeKey && !s.hidden[s.Key.Column] {
		out = append(out, s.Key)
	}
	return append(out, s.FormFields()...)
}

func (s *Schema) ListFields() []Field {
	if len(s.listOnly) > 0 {
		out := make([]Field, 0, len(s.listOnly))
		for _, column := range s.listOnly {
			if f, ok := s.Field(column); ok {
				out = append(out, f)
			}
		}
		return out
	}
	out := make([]Field, 0, listColumnLimit)
	for _, f := range s.Visible() {
		if f.Sensitive || f.Kind == KindBytes {
			continue
		}
		out = append(out, f)
		if len(out) == listColumnLimit {
			break
		}
	}
	return out
}

func (s *Schema) Columns() []string {
	out := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		out = append(out, f.Column)
	}
	return out
}

func (s *Schema) SetListColumns(columns []string) { s.listOnly = columns }

func (s *Schema) Hide(columns []string) {
	if s.hidden == nil {
		s.hidden = make(map[string]bool, len(columns))
	}
	for _, c := range columns {
		s.hidden[c] = true
	}
}

func (s *Schema) SortColumn(candidate string) (string, string, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", "", false
	}

	direction := "asc"
	if strings.HasPrefix(candidate, "-") {
		candidate, direction = strings.TrimPrefix(candidate, "-"), "desc"
	}
	if name, suffix, found := strings.Cut(candidate, " "); found {
		candidate = name
		switch strings.ToLower(strings.TrimSpace(suffix)) {
		case "desc":
			direction = "desc"
		case "asc":
			direction = "asc"
		default:
			return "", "", false
		}
	}

	candidate = strings.ToLower(strings.TrimSpace(candidate))
	for _, field := range s.Fields {
		if field.Column == candidate {
			return field.Column, direction, true
		}
	}
	return "", "", false
}

type Ordering struct {
	Column string
	Desc   bool
}

func (s *Schema) OrderBy(candidate string) []Ordering {
	out := make([]Ordering, 0, 2)
	seen := map[string]bool{}
	for _, part := range strings.Split(candidate, ",") {
		column, direction, ok := s.SortColumn(part)
		if !ok || seen[column] {
			continue
		}
		seen[column] = true
		out = append(out, Ordering{Column: column, Desc: direction == "desc"})
	}
	return out
}

func (s *Schema) Relation(column string) (Relation, bool) {
	for _, relation := range s.Relations {
		if relation.Column == column {
			return relation, true
		}
	}
	return Relation{}, false
}

func (s *Schema) HasColumn(name string) bool {
	for _, field := range s.Fields {
		if field.Column == name {
			return true
		}
	}
	return false
}
