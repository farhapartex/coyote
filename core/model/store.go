package model

import "context"

type Query struct {
	Limit  int
	Offset int
	Order  string
	Search string
}

type Page struct {
	Records []Record
	Total   int64
}

type Store interface {
	List(ctx context.Context, schema *Schema, query Query) (Page, error)
	Find(ctx context.Context, schema *Schema, id string) (Record, error)
	Insert(ctx context.Context, schema *Schema, record Record) (string, error)
	Update(ctx context.Context, schema *Schema, id string, record Record) error
	Delete(ctx context.Context, schema *Schema, id string) error
}
