package migrate

import "sort"

type Change struct {
	Ops      []Op
	Warnings []string
}

func (c Change) Empty() bool { return len(c.Ops) == 0 }

func Diff(previous, desired Snapshot) Change {
	var change Change

	for _, table := range desired.Tables {
		before, existed := previous.Table(table.Name)
		if !existed {
			change.Ops = append(change.Ops, CreateTable{Table: table})
			continue
		}
		change.merge(diffColumns(before, table))
		change.merge(diffIndexes(before, table))
	}

	for _, table := range previous.Tables {
		if _, kept := desired.Table(table.Name); !kept {
			change.Ops = append(change.Ops, DropTable{Name: table.Name})
			change.Warnings = append(change.Warnings,
				"table "+table.Name+" is no longer declared; the generated drop will destroy its data")
		}
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
	return change
}

func diffIndexes(before, after Table) Change {
	var change Change
	for _, index := range after.Indexes {
		if _, ok := before.Index(index.Name); !ok {
			change.Ops = append(change.Ops, CreateIndex{
				Table: after.Name, Name: index.Name, Columns: index.Columns, Unique: index.Unique,
			})
		}
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
