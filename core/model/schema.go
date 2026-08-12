package model

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
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

func (f Field) Editable() bool { return !f.Generated && !f.PrimaryKey }

type Schema struct {
	Table    string
	Slug     string
	Label    string
	Plural   string
	Fields   []Field
	Indexes  []Index
	Key      Field
	listOnly []string
	hidden   map[string]bool
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
