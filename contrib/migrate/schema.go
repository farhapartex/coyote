package migrate

import (
	"sort"

	"github.com/farhapartex/coyote/core/model"
)

type Column struct {
	Name          string     `json:"name"`
	Kind          model.Kind `json:"kind"`
	Size          int        `json:"size,omitempty"`
	NotNull       bool       `json:"notNull,omitempty"`
	PrimaryKey    bool       `json:"primaryKey,omitempty"`
	AutoIncrement bool       `json:"autoIncrement,omitempty"`
	Default       string     `json:"default,omitempty"`
}

type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Indexes []Index  `json:"indexes,omitempty"`
}

func (t Table) Column(name string) (Column, bool) {
	for _, c := range t.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}

func (t Table) Index(name string) (Index, bool) {
	for _, ix := range t.Indexes {
		if ix.Name == name {
			return ix, true
		}
	}
	return Index{}, false
}

func TableOf(schema *model.Schema) Table {
	table := Table{Name: schema.Table}
	for _, f := range schema.Fields {
		table.Columns = append(table.Columns, Column{
			Name:          f.Column,
			Kind:          f.Kind,
			Size:          f.Size,
			NotNull:       f.Required || f.PrimaryKey,
			PrimaryKey:    f.PrimaryKey,
			AutoIncrement: f.AutoIncrement,
			Default:       f.Default,
		})
	}
	for _, ix := range schema.Indexes {
		table.Indexes = append(table.Indexes, Index{Name: ix.Name, Columns: ix.Columns, Unique: ix.Unique})
	}
	sort.Slice(table.Indexes, func(i, j int) bool { return table.Indexes[i].Name < table.Indexes[j].Name })
	return table
}
