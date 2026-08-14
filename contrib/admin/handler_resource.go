package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
	"github.com/farhapartex/coyote/core/view"
)

const pageSize = 25

type listRow struct {
	ID    string
	Cells []string
}

func (a *Admin) resourceList(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, ok := a.storeFor(w, r)
		if !ok {
			return
		}

		page := 1
		if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 1 {
			page = n
		}
		result, err := records.List(r.Context(), entry.schema, model.Query{
			Limit:  pageSize,
			Offset: (page - 1) * pageSize,
			Order:  entry.order,
		})
		if err != nil {
			a.fail(w, r, err)
			return
		}

		columns := entry.schema.ListFields()
		rows := make([]listRow, 0, len(result.Records))
		for _, record := range result.Records {
			cells := make([]string, 0, len(columns))
			for _, column := range columns {
				cells = append(cells, record.String(column.Column))
			}
			rows = append(rows, listRow{ID: record.String(entry.schema.Key.Column), Cells: cells})
		}

		a.render(w, r, http.StatusOK, "resource_list.html", view.Data{
			"Nav":      entry.Slug(),
			"Resource": entry,
			"Columns":  columns,
			"Rows":     rows,
			"Total":    result.Total,
			"ReadOnly": entry.readOnly,
			"Page":     page,
			"HasPrev":  page > 1,
			"HasNext":  int64(page*pageSize) < result.Total,
			"PrevPage": page - 1,
			"NextPage": page + 1,
		})
	}
}

func (a *Admin) resourceForm(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, ok := a.storeFor(w, r)
		if !ok {
			return
		}

		id := r.PathValue("id")
		var record model.Record
		if id != "" {
			found, err := records.Find(r.Context(), entry.schema, id)
			if err != nil {
				a.notFoundOrFail(w, r, err)
				return
			}
			record = found
		}
		a.renderForm(w, r, http.StatusOK, entry, record, id, nil)
	}
}

func (a *Admin) resourceCreate(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, ok := a.writableStore(w, r, entry)
		if !ok {
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		bound := bindForm(r, entry.schema, true)
		if !bound.valid() {
			a.renderForm(w, r, http.StatusBadRequest, entry, bound.Record, "", bound.Errors)
			return
		}
		if _, err := records.Insert(r.Context(), entry.schema, bound.Record); err != nil {
			a.renderForm(w, r, http.StatusBadRequest, entry, bound.Record, "", map[string]string{"": err.Error()})
			return
		}
		view.Success(r, entry.schema.Label+" created.")
		view.Redirect(w, r, a.prefix+"/"+entry.Slug())
	}
}

func (a *Admin) resourceUpdate(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, ok := a.writableStore(w, r, entry)
		if !ok {
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		id := r.PathValue("id")
		bound := bindForm(r, entry.schema, false)
		if !bound.valid() {
			a.renderForm(w, r, http.StatusBadRequest, entry, bound.Record, id, bound.Errors)
			return
		}
		if err := records.Update(r.Context(), entry.schema, id, bound.Record); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				a.notFound(w, r)
				return
			}
			a.renderForm(w, r, http.StatusBadRequest, entry, bound.Record, id, map[string]string{"": err.Error()})
			return
		}
		view.Success(r, entry.schema.Label+" updated.")
		view.Redirect(w, r, a.prefix+"/"+entry.Slug())
	}
}

func (a *Admin) resourceDelete(entry managed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, ok := a.writableStore(w, r, entry)
		if !ok {
			return
		}
		if err := records.Delete(r.Context(), entry.schema, r.PathValue("id")); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				a.notFound(w, r)
				return
			}
			view.Error(r, err.Error())
		} else {
			view.Success(r, entry.schema.Label+" deleted.")
		}
		view.Redirect(w, r, a.prefix+"/"+entry.Slug())
	}
}

func (a *Admin) renderForm(w http.ResponseWriter, r *http.Request, status int, entry managed, record model.Record, id string, errs map[string]string) {
	if errs == nil {
		errs = map[string]string{}
	}
	action := a.prefix + "/" + entry.Slug() + "/new"
	if id != "" {
		action = a.prefix + "/" + entry.Slug() + "/" + id
	}
	a.render(w, r, status, "resource_form.html", view.Data{
		"Nav":      entry.Slug(),
		"Resource": entry,
		"Fields":   buildForm(entry.schema, record, errs, entry.readOnly),
		"IsNew":    id == "",
		"RecordID": id,
		"Action":   action,
		"Error":    errs[""],
		"ReadOnly": entry.readOnly,
	})
}
