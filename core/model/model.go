package model

type Model struct {
	Entity any
	Table  string
	Alias  string
}

func Of(entity any) Model {
	return Model{Entity: entity}
}

func Named(table string, entity any) Model {
	return Model{Entity: entity, Table: table}
}

func On(alias string, entity any) Model {
	return Model{Entity: entity, Alias: alias}
}

func (m Model) NamedOn(alias, table string) Model {
	m.Alias, m.Table = alias, table
	return m
}
