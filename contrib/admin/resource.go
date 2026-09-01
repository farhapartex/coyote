package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/auth"
)

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
	Name       string
	Label      string
	Confirm    string
	Permission string
	Run        func(r *http.Request, ids []string) (int, error)
}

func (a Action) permission() string {
	if a.Permission == "" {
		return auth.ActionUpdate
	}
	return a.Permission
}

type Actionable interface {
	Actions() []Action
}
