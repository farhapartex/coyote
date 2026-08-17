package model

import (
	"context"

	"gorm.io/gorm"
)

type Op string

const (
	Eq      Op = "eq"
	Ne      Op = "ne"
	Lt      Op = "lt"
	Lte     Op = "lte"
	Gt      Op = "gt"
	Gte     Op = "gte"
	In      Op = "in"
	Like    Op = "like"
	Null    Op = "null"
	NotNull Op = "notnull"
)

type Filter struct {
	Column string
	Op     Op
	Value  any
}

type Query struct {
	Limit   int
	Offset  int
	Order   string
	Sort    string
	Select  []string
	Filters []Filter
	With    []string
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
	Count(ctx context.Context, schema *Schema, query Query) (int64, error)
	Exists(ctx context.Context, schema *Schema, query Query) (bool, error)
	First(ctx context.Context, schema *Schema, query Query) (Record, error)
	WithTx(tx *gorm.DB) Store
}
