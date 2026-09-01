package migrate

import (
	"fmt"
	"sort"

	"github.com/farhapartex/coyote/core/model"
)

type Change struct {
	Ops      []Op
	Warnings []string
}

func (c Change) Empty() bool { return len(c.Ops) == 0 }

func Diff(previous, desired Snapshot) Change {
	var change Change

	created := []Table{}
	for _, table := range desired.Tables {
		before, existed := previous.Table(table.Name)
		if !existed {
			created = append(created, table)
			continue
		}
		change.merge(diffColumns(before, table))
		change.merge(diffIndexes(before, table))
	}

	creates := make([]Op, 0, len(created))
	for _, table := range orderedForCreate(created) {
		creates = append(creates, CreateTable{Table: table})
	}
	change.Ops = append(creates, change.Ops...)

	dropped := []Table{}
	for _, table := range previous.Tables {
		if _, kept := desired.Table(table.Name); !kept {
			dropped = append(dropped, table)
		}
	}
	ordered := orderedForCreate(dropped)
	for i := len(ordered) - 1; i >= 0; i-- {
		change.Ops = append(change.Ops, DropTable{Name: ordered[i].Name})
		change.Warnings = append(change.Warnings,
			"table "+ordered[i].Name+" is no longer declared; the generated drop will destroy its data")
	}
	return change
}

func (c *Change) merge(other Change) {
	c.Ops = append(c.Ops, other.Ops...)
	c.Warnings = append(c.Warnings, other.Warnings...)
}

func diffColumns(before, after Table) Change {
	var change Change
	var added, removed []Column

	for _, column := range after.Columns {
		if _, ok := before.Column(column.Name); !ok {
			added = append(added, column)
		}
	}
	for _, column := range before.Columns {
		if _, ok := after.Column(column.Name); !ok {
			removed = append(removed, column)
		}
	}

	for _, column := range added {
		change.Ops = append(change.Ops, AddColumn{Table: after.Name, Column: column})
		if column.NotNull && column.Default == "" && !column.PrimaryKey {
			change.Warnings = append(change.Warnings,
				after.Name+"."+column.Name+" is NOT NULL without a default; existing rows need a value")
		}
	}
	for _, column := range removed {
		change.Ops = append(change.Ops, DropColumn{Table: after.Name, Column: column.Name})
	}
	if len(added) == 1 && len(removed) == 1 && added[0].Kind == removed[0].Kind {
		change.Warnings = append(change.Warnings,
			"if "+after.Name+"."+removed[0].Name+" became "+added[0].Name+
				", replace the drop and add with migrate.RenameColumn to keep the data")
	}

	change.merge(diffChangedColumns(before, after))
	return change
}

func diffChangedColumns(before, after Table) Change {
	var change Change

	for _, column := range after.Columns {
		previous, ok := before.Column(column.Name)
		if !ok || previous == column {
			continue
		}
		change.Ops = append(change.Ops, AlterColumn{
			Table:   after.Name,
			From:    previous,
			To:      column,
			Columns: after.Columns,
			Indexes: after.Indexes,
		})
		change.Warnings = append(change.Warnings, alterWarnings(after.Name, previous, column)...)
	}
	return change
}

func alterWarnings(table string, from, to Column) []string {
	out := []string{}
	name := table + "." + to.Name

	if from.Size > 0 && to.Size > 0 && to.Size < from.Size {
		out = append(out, fmt.Sprintf("%s narrows from %d to %d characters; longer values will be truncated or rejected",
			name, from.Size, to.Size))
	}
	if from.Kind != to.Kind && !wideningKind(from.Kind, to.Kind) {
		out = append(out, fmt.Sprintf("%s changes type from %s to %s; existing values may not convert",
			name, from.Kind, to.Kind))
	}
	if to.NotNull && !from.NotNull && to.Default == "" {
		out = append(out, name+" becomes NOT NULL without a default; rows holding null will fail")
	}
	return out
}

func wideningKind(from, to model.Kind) bool {
	switch {
	case from == to:
		return true
	case from == model.KindInt && to == model.KindFloat:
		return true
	case from == model.KindString && to == model.KindText:
		return true
	}
	return false
}

func diffIndexes(before, after Table) Change {
	var change Change
	for _, index := range after.Indexes {
		existing, ok := before.Index(index.Name)
		if !ok {
			change.Ops = append(change.Ops, CreateIndex{
				Table: after.Name, Name: index.Name, Columns: index.Columns, Unique: index.Unique,
			})
			continue
		}
		if sameIndexDefinition(existing, index) {
			continue
		}
		change.Ops = append(change.Ops,
			DropIndex{Table: after.Name, Name: index.Name},
			CreateIndex{
				Table: after.Name, Name: index.Name, Columns: index.Columns, Unique: index.Unique,
			})
	}
	for _, index := range before.Indexes {
		if _, ok := after.Index(index.Name); !ok {
			change.Ops = append(change.Ops, DropIndex{Table: after.Name, Name: index.Name})
		}
	}
	sort.SliceStable(change.Ops, func(i, j int) bool {
		_, first := change.Ops[i].(DropIndex)
		_, second := change.Ops[j].(DropIndex)
		return first && !second
	})
	return change
}

func sameIndexDefinition(before, after Index) bool {
	if before.Unique != after.Unique || len(before.Columns) != len(after.Columns) {
		return false
	}
	for i := range before.Columns {
		if before.Columns[i] != after.Columns[i] {
			return false
		}
	}
	return true
}
