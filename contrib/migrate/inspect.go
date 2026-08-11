package migrate

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
)

type inspector struct {
	handle *gorm.DB
}

func (i inspector) tableName(m model.Model) (string, error) {
	if m.Table != "" {
		return m.Table, nil
	}
	statement := &gorm.Statement{DB: i.handle}
	if err := statement.Parse(m.Entity); err != nil {
		return "", fmt.Errorf("coyote/migrate: parsing %T: %w", m.Entity, err)
	}
	return statement.Schema.Table, nil
}

func (i inspector) expectedColumns(m model.Model) ([]string, error) {
	statement := &gorm.Statement{DB: i.handle}
	if err := statement.Parse(m.Entity); err != nil {
		return nil, fmt.Errorf("coyote/migrate: parsing %T: %w", m.Entity, err)
	}
	return statement.Schema.DBNames, nil
}

func (i inspector) existingColumns(m model.Model) ([]string, error) {
	types, err := i.handle.Migrator().ColumnTypes(m.Entity)
	if err != nil {
		return nil, fmt.Errorf("coyote/migrate: reading columns for %T: %w", m.Entity, err)
	}
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, t.Name())
	}
	return out, nil
}

func (i inspector) change(m model.Model) (TableChange, error) {
	table, err := i.tableName(m)
	if err != nil {
		return TableChange{}, err
	}
	if !i.handle.Migrator().HasTable(m.Entity) {
		return TableChange{Table: table, Created: true}, nil
	}

	expected, err := i.expectedColumns(m)
	if err != nil {
		return TableChange{}, err
	}
	existing, err := i.existingColumns(m)
	if err != nil {
		return TableChange{}, err
	}
	present := make(map[string]bool, len(existing))
	for _, name := range existing {
		present[name] = true
	}

	change := TableChange{Table: table}
	for _, name := range expected {
		if !present[name] {
			change.AddedColumns = append(change.AddedColumns, name)
		}
	}
	return change, nil
}
