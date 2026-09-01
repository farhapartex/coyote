package migrate

import (
	"fmt"
	"sort"
)

func (t Table) clone() Table {
	out := Table{Name: t.Name}
	out.Columns = append(out.Columns, t.Columns...)
	for _, ix := range t.Indexes {
		copied := Index{Name: ix.Name, Unique: ix.Unique}
		copied.Columns = append(copied.Columns, ix.Columns...)
		out.Indexes = append(out.Indexes, copied)
	}
	return out
}

func (s Snapshot) clone() Snapshot {
	out := Snapshot{Version: s.Version}
	for _, t := range s.Tables {
		out.Tables = append(out.Tables, t.clone())
	}
	return out
}

func (s Snapshot) Apply(ops []Op) (Snapshot, error) {
	next := s.clone()
	for _, op := range ops {
		if err := next.apply(op); err != nil {
			return Snapshot{}, err
		}
	}
	sort.Slice(next.Tables, func(i, j int) bool { return next.Tables[i].Name < next.Tables[j].Name })
	return next, nil
}

func (s *Snapshot) apply(op Op) error {
	switch o := op.(type) {
	case CreateTable:
		if _, ok := s.Table(o.Table.Name); ok {
			return fmt.Errorf("coyote/migrate: %s is already in the snapshot", o.Table.Name)
		}
		s.Tables = append(s.Tables, o.Table.clone())
		return nil
	case DropTable:
		return s.removeTable(o.Name)
	case AddColumn:
		return s.withTable(o.Table, func(t *Table) error {
			if _, ok := t.Column(o.Column.Name); ok {
				return fmt.Errorf("coyote/migrate: %s.%s is already in the snapshot", o.Table, o.Column.Name)
			}
			t.Columns = append(t.Columns, o.Column)
			return nil
		})
	case DropColumn:
		return s.withTable(o.Table, func(t *Table) error {
			return removeColumn(t, o.Table, o.Column)
		})
	case RenameColumn:
		return s.withTable(o.Table, func(t *Table) error {
			return renameColumn(t, o.Table, o.From, o.To)
		})
	case AlterColumn:
		return s.withTable(o.Table, func(t *Table) error {
			return replaceColumn(t, o.Table, o.From.Name, o.To)
		})
	case CreateIndex:
		return s.withTable(o.Table, func(t *Table) error {
			if _, ok := t.Index(o.Name); ok {
				return fmt.Errorf("coyote/migrate: index %s is already in the snapshot", o.Name)
			}
			t.Indexes = append(t.Indexes, Index{Name: o.Name, Columns: append([]string(nil), o.Columns...), Unique: o.Unique})
			return nil
		})
	case DropIndex:
		return s.withTable(o.Table, func(t *Table) error {
			return removeIndex(t, o.Name)
		})
	}
	return fmt.Errorf("coyote/migrate: %s cannot be replayed against the snapshot", op.Describe())
}

func (s *Snapshot) withTable(name string, mutate func(*Table) error) error {
	for i := range s.Tables {
		if s.Tables[i].Name == name {
			return mutate(&s.Tables[i])
		}
	}
	return fmt.Errorf("coyote/migrate: %s is not in the snapshot", name)
}

func (s *Snapshot) removeTable(name string) error {
	for i := range s.Tables {
		if s.Tables[i].Name == name {
			s.Tables = append(s.Tables[:i], s.Tables[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("coyote/migrate: %s is not in the snapshot", name)
}

func removeColumn(t *Table, table, column string) error {
	for i := range t.Columns {
		if t.Columns[i].Name == column {
			t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("coyote/migrate: %s.%s is not in the snapshot", table, column)
}

func renameColumn(t *Table, table, from, to string) error {
	for i := range t.Columns {
		if t.Columns[i].Name == from {
			t.Columns[i].Name = to
			return nil
		}
	}
	return fmt.Errorf("coyote/migrate: %s.%s is not in the snapshot", table, from)
}

func replaceColumn(t *Table, table, name string, with Column) error {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			t.Columns[i] = with
			return nil
		}
	}
	return fmt.Errorf("coyote/migrate: %s.%s is not in the snapshot", table, name)
}

func removeIndex(t *Table, name string) error {
	for i := range t.Indexes {
		if t.Indexes[i].Name == name {
			t.Indexes = append(t.Indexes[:i], t.Indexes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("coyote/migrate: index %s is not in the snapshot", name)
}
