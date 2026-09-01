package view

import (
	"net/http"
	"strconv"

	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/model"
)

type Renderer interface {
	RenderStatus(w http.ResponseWriter, r *http.Request, status int, page string, data Data)
}

type Options struct {
	Store     model.Store
	Schema    *model.Schema
	Template  string
	Renderer  Renderer
	PerPage   int
	Paginator Paginator
	Order     string
	Fields    []string
	Exclude   []string
	Filter    func(*http.Request, model.Query) model.Query
	Allow     func(*http.Request) bool
	Data      func(*http.Request, Data) Data
	Redirect  string
	IDParam   string
}

func (o Options) bindable() []model.Field {
	return o.Schema.BindableFields(o.Fields, o.Exclude)
}

func (o Options) idOf(r *http.Request) string {
	name := o.IDParam
	if name == "" {
		name = "id"
	}
	return r.PathValue(name)
}

func (o Options) query(r *http.Request, base model.Query) model.Query {
	if o.Filter != nil {
		return o.Filter(r, base)
	}
	return base
}

func (o Options) extend(r *http.Request, data Data) Data {
	if o.Data != nil {
		return o.Data(r, data)
	}
	return data
}

func (o Options) render(w http.ResponseWriter, r *http.Request, status int, data Data) {
	o.Renderer.RenderStatus(w, r, status, o.Template, o.extend(r, data))
}

func (o Options) allowed(w http.ResponseWriter, r *http.Request) bool {
	if o.Allow == nil || o.Allow(r) {
		return true
	}
	http.Error(w, "403 forbidden", http.StatusForbidden)
	return false
}

func List(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !opts.allowed(w, r) {
			return
		}

		number := 1
		if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 1 {
			number = n
		}
		offset := 0
		if opts.PerPage > 0 && number > 1 {
			offset = (number - 1) * opts.PerPage
		}

		result, err := opts.Store.List(r.Context(), opts.Schema, opts.query(r, model.Query{
			Limit:  opts.PerPage,
			Offset: offset,
			Order:  opts.Order,
			Sort:   r.URL.Query().Get("sort"),
		}))
		if err != nil {
			fail(w)
			return
		}

		page := Paginate(opts.Paginator, result.Total, number, opts.PerPage)
		if page.Offset != offset {
			result, err = opts.Store.List(r.Context(), opts.Schema, opts.query(r, model.Query{
				Limit:  page.Limit,
				Offset: page.Offset,
				Order:  opts.Order,
				Sort:   r.URL.Query().Get("sort"),
			}))
			if err != nil {
				fail(w)
				return
			}
		}

		opts.render(w, r, http.StatusOK, Data{
			"Records": result.Records,
			"Total":   result.Total,
			"Page":    page,
			"Sort":    r.URL.Query().Get("sort"),
		})
	}
}

func Detail(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !opts.allowed(w, r) {
			return
		}
		record, err := opts.Store.Find(r.Context(), opts.Schema, opts.idOf(r))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		opts.render(w, r, http.StatusOK, Data{"Record": record})
	}
}

func Create(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !opts.allowed(w, r) {
			return
		}
		if r.Method == http.MethodGet {
			opts.render(w, r, http.StatusOK, Data{"Record": model.Record{}, "IsNew": true})
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		bound := form.RecordOf(r.PostForm, opts.Schema, opts.bindable(), true)
		if !bound.Valid() {
			opts.render(w, r, http.StatusUnprocessableEntity, Data{
				"Record": bound.Record, "IsNew": true, "Problems": bound.Problems,
			})
			return
		}
		_, err := opts.Store.Insert(r.Context(), opts.Schema, bound.Record)
		if err != nil {
			opts.render(w, r, http.StatusUnprocessableEntity, Data{
				"Record": bound.Record, "IsNew": true, "Error": err.Error(),
			})
			return
		}
		Redirect(w, r, opts.after())
	}
}

func Update(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !opts.allowed(w, r) {
			return
		}
		id := opts.idOf(r)
		if r.Method == http.MethodGet {
			record, err := opts.Store.Find(r.Context(), opts.Schema, id)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			opts.render(w, r, http.StatusOK, Data{"Record": record, "IsNew": false})
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		bound := form.RecordOf(r.PostForm, opts.Schema, opts.bindable(), false)
		if !bound.Valid() {
			opts.render(w, r, http.StatusUnprocessableEntity, Data{
				"Record": bound.Record, "IsNew": false, "Problems": bound.Problems,
			})
			return
		}
		if err := opts.Store.Update(r.Context(), opts.Schema, id, bound.Record); err != nil {
			opts.render(w, r, http.StatusUnprocessableEntity, Data{
				"Record": bound.Record, "IsNew": false, "Error": err.Error(),
			})
			return
		}
		Redirect(w, r, opts.after())
	}
}

func Delete(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !opts.allowed(w, r) {
			return
		}
		if err := opts.Store.Delete(r.Context(), opts.Schema, opts.idOf(r)); err != nil {
			fail(w)
			return
		}
		Redirect(w, r, opts.after())
	}
}

func (o Options) after() string {
	if o.Redirect == "" {
		return "/"
	}
	return o.Redirect
}

func fail(w http.ResponseWriter) {
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
}
