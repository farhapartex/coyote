package migrate

import "fmt"

type TableChange struct {
	Table        string
	Created      bool
	AddedColumns []string
}

func (c TableChange) Empty() bool {
	return !c.Created && len(c.AddedColumns) == 0
}

func (c TableChange) String() string {
	if c.Created {
		return fmt.Sprintf("create table %s", c.Table)
	}
	return fmt.Sprintf("alter table %s add %v", c.Table, c.AddedColumns)
}

type Report struct {
	Changes []TableChange
}

func (r Report) Empty() bool { return len(r.Changes) == 0 }

func (r Report) Tables() []string {
	out := make([]string, 0, len(r.Changes))
	for _, c := range r.Changes {
		out = append(out, c.Table)
	}
	return out
}
