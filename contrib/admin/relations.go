package admin

import (
	"net/http"

	"github.com/farhapartex/coyote/core/model"
)

const optionLimit = 200

type option struct {
	Value    string
	Label    string
	Selected bool
}

func (a *Admin) relationOptions(r *http.Request, entry managed, column, current string) []option {
	relation, found := entry.schema.Relation(column)
	if !found {
		return nil
	}
	records, ok := a.storeQuietly(r)
	if !ok {
		return nil
	}
	optioner, ok := records.(model.Optioner)
	if !ok {
		return nil
	}

	rows, err := optioner.Options(r.Context(), relation, optionLimit)
	if err != nil {
		a.app.Logger.Error("admin could not list options", "relation", relation.Target, "error", err)
		return nil
	}

	out := make([]option, 0, len(rows)+1)
	for _, row := range rows {
		value := row.String(relation.TargetKey)
		label := row.String(relation.LabelColumn)
		if label == "" {
			label = value
		}
		out = append(out, option{Value: value, Label: label, Selected: value == current})
	}
	return out
}

func (a *Admin) storeQuietly(r *http.Request) (model.Store, bool) {
	records, err := a.app.Store()
	if err != nil {
		return nil, false
	}
	return records, true
}
