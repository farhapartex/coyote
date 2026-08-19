package admin

import (
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/model"
)

type filterChoice struct {
	Column string
	Label  string
	Value  string
	Any    bool
	Yes    bool
	No     bool
}

func (a *Admin) listQuery(r *http.Request, entry managed) (model.Query, string, []filterChoice) {
	query := model.Query{Order: entry.order}

	if column, direction, ok := entry.schema.SortColumn(r.URL.Query().Get("sort")); ok {
		query.Sort = column
		if direction == "desc" {
			query.Sort = "-" + column
		}
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if term != "" && entry.Searchable() {
		pattern := "%" + escapeLike(term) + "%"
		for _, column := range entry.searchOn {
			query.AnyOf = append(query.AnyOf, model.Filter{Column: column, Op: model.Like, Value: pattern})
		}
	}

	choices := make([]filterChoice, 0, len(entry.filterOn))
	for _, column := range entry.filterOn {
		field, ok := entry.schema.Field(column)
		if !ok {
			continue
		}
		value := r.URL.Query().Get("filter." + column)
		choice := filterChoice{
			Column: column,
			Label:  field.Label,
			Value:  value,
			Any:    value == "",
			Yes:    value == "yes",
			No:     value == "no",
		}
		switch value {
		case "yes":
			query.Filters = append(query.Filters, model.Filter{Column: column, Op: model.Eq, Value: true})
		case "no":
			query.Filters = append(query.Filters, model.Filter{Column: column, Op: model.Eq, Value: false})
		}
		choices = append(choices, choice)
	}

	for _, relation := range entry.relations {
		if entry.schema.HasColumn(relation.Column) {
			query.With = append(query.With, relation.Column)
		}
	}
	return query, term, choices
}

func escapeLike(term string) string {
	term = strings.ReplaceAll(term, `\`, `\\`)
	term = strings.ReplaceAll(term, "%", `\%`)
	return strings.ReplaceAll(term, "_", `\_`)
}

func (a *Admin) sortLinks(r *http.Request, entry managed) map[string]string {
	current := r.URL.Query().Get("sort")
	out := map[string]string{}

	for _, field := range entry.schema.ListFields() {
		next := field.Column
		if current == field.Column {
			next = "-" + field.Column
		}
		out[field.Column] = next
	}
	return out
}

func displayCell(record model.Record, column string) string {
	if label := record.String(column + "__label"); label != "" {
		return label
	}
	return record.String(column)
}
