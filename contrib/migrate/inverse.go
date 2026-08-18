package migrate

import "fmt"

type Reversible interface {
	Op
	Inverse() (Op, bool)
}

func (o CreateTable) Inverse() (Op, bool) {
	return DropTable{Name: o.Table.Name, Restore: o.Table}, true
}

func (o DropTable) Inverse() (Op, bool) {
	if o.Restore.Name == "" {
		return nil, false
	}
	return CreateTable{Table: o.Restore}, true
}

func (o AddColumn) Inverse() (Op, bool) {
	return DropColumn{Table: o.Table, Column: o.Column.Name, Restore: o.Column}, true
}

func (o DropColumn) Inverse() (Op, bool) {
	if o.Restore.Name == "" {
		return nil, false
	}
	return AddColumn{Table: o.Table, Column: o.Restore}, true
}

func (o RenameColumn) Inverse() (Op, bool) {
	return RenameColumn{Table: o.Table, From: o.To, To: o.From}, true
}

func (o CreateIndex) Inverse() (Op, bool) {
	return DropIndex{Table: o.Table, Name: o.Name, Columns: o.Columns, Unique: o.Unique}, true
}

func (o DropIndex) Inverse() (Op, bool) {
	if len(o.Columns) == 0 {
		return nil, false
	}
	return CreateIndex{Table: o.Table, Name: o.Name, Columns: o.Columns, Unique: o.Unique}, true
}

func Invert(ops []Op) ([]Op, []string) {
	out := make([]Op, 0, len(ops))
	blocked := []string{}

	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		reversible, ok := op.(Reversible)
		if !ok {
			blocked = append(blocked, op.Describe())
			continue
		}
		inverse, ok := reversible.Inverse()
		if !ok {
			blocked = append(blocked, op.Describe())
			continue
		}
		out = append(out, inverse)
	}
	return out, blocked
}

func Destructive(ops []Op) []string {
	out := []string{}
	for _, op := range ops {
		switch typed := op.(type) {
		case DropTable:
			out = append(out, fmt.Sprintf("every row in %s", typed.Name))
		case DropColumn:
			out = append(out, fmt.Sprintf("every value in %s.%s", typed.Table, typed.Column))
		}
	}
	return out
}
