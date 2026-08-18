package admin

import "net/http"

type Resource interface {
	Entity() any
}

type Labelled interface {
	Label() string
}

type Pluralised interface {
	PluralLabel() string
}

type Routed interface {
	Slug() string
}

type Listed interface {
	ListColumns() []string
}

type Concealed interface {
	HiddenColumns() []string
}

type Sorted interface {
	DefaultOrder() string
}

type Guarded interface {
	ReadOnly() bool
}

type Searchable interface {
	SearchColumns() []string
}

type Filterable interface {
	FilterColumns() []string
}

type Action struct {
	Name    string
	Label   string
	Confirm string
	Run     func(r *http.Request, ids []string) (int, error)
}

type Actionable interface {
	Actions() []Action
}
