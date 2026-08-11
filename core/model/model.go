package model

type Model struct {
	Entity any
	Table  string
}

func Of(entity any) Model {
	return Model{Entity: entity}
}

func Named(table string, entity any) Model {
	return Model{Entity: entity, Table: table}
}
