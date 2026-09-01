package migrate

func parentsOf(table Table) []string {
	out := []string{}
	for _, column := range table.Columns {
		if column.References.IsZero() || column.References.Table == table.Name {
			continue
		}
		out = append(out, column.References.Table)
	}
	return out
}

func orderedForCreate(tables []Table) []Table {
	pending := make(map[string]bool, len(tables))
	for _, table := range tables {
		pending[table.Name] = true
	}

	out := make([]Table, 0, len(tables))
	placed := make(map[string]bool, len(tables))

	for len(out) < len(tables) {
		progressed := false
		for _, table := range tables {
			if placed[table.Name] {
				continue
			}
			ready := true
			for _, parent := range parentsOf(table) {
				if pending[parent] && !placed[parent] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			out = append(out, table)
			placed[table.Name] = true
			progressed = true
		}
		if progressed {
			continue
		}
		for _, table := range tables {
			if !placed[table.Name] {
				out = append(out, table)
				placed[table.Name] = true
			}
		}
	}
	return out
}
