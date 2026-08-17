package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
	"github.com/farhapartex/coyote/core/upload"
	"github.com/farhapartex/coyote/core/view"
)

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

		number := 1
		if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 1 {
			number = n
		}

		paginator := a.app.Settings.Pagination.Paginator
		perPage := a.app.Settings.Pagination.PerPage

		offset := 0
		if perPage > 0 && number > 1 {
			offset = (number - 1) * perPage
		}

		result, err := records.List(r.Context(), entry.schema, model.Query{
			Limit:  perPage,
			Offset: offset,
			Order:  entry.order,
		})
		if err != nil {
			a.fail(w, r, err)
			return
		}

		page := view.Paginate(paginator, result.Total, number, perPage)
		if page.Offset != offset {
			result, err = records.List(r.Context(), entry.schema, model.Query{
				Limit:  page.Limit,
				Offset: page.Offset,
				Order:  entry.order,
			})
			if err != nil {
				a.fail(w, r, err)
				return
			}
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
		if err := form.Parse(r, uploadMemory); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		bound := form.Record(r.PostForm, entry.schema, true)
		problems := bound.Errors()
		for column, message := range a.attachFiles(r, entry, bound.Record, nil) {
			problems[column] = message
		}
		if len(problems) > 0 {
			a.renderForm(w, r, http.StatusBadRequest, entry, bound.Record, "", problems)
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
		if err := form.Parse(r, uploadMemory); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		id := r.PathValue("id")
		bound := form.Record(r.PostForm, entry.schema, false)
		problems := bound.Errors()

		var existing model.Record
		if a.hasFiles(entry.schema) {
			if found, err := records.Find(r.Context(), entry.schema, id); err == nil {
				existing = found
			}
		}
		for column, message := range a.attachFiles(r, entry, bound.Record, existing) {
			problems[column] = message
		}
		if len(problems) > 0 {
			a.renderForm(w, r, http.StatusBadRequest, entry, bound.Record, id, problems)
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
	fields := buildForm(entry.schema, record, errs, entry.readOnly)
	for i, field := range fields {
		if field.Type == "file" && field.Value != "" {
			fields[i].URL = a.app.MediaURL(upload.Ref(field.Value))
		}
	}

	a.render(w, r, status, "resource_form.html", view.Data{
		"Nav":      entry.Slug(),
		"Resource": entry,
		"Fields":   fields,
		"IsNew":    id == "",
		"RecordID": id,
		"Action":   action,
		"Error":    errs[""],
		"ReadOnly": entry.readOnly,
		"Uploads":  a.app.Uploads != nil,
	})
}
