package cli

import (
	"github.com/farhapartex/coyote/core/model"
)

func schemasOf(app Application) ([]*model.Schema, error) {
	handle, err := app.DB()
	if err != nil {
		return nil, err
	}
	models := app.Models()
	out := make([]*model.Schema, 0, len(models))
	for _, m := range models {
		schema, err := model.Describe(handle, m.Entity)
		if err != nil {
			return nil, err
		}
		if m.Table != "" {
			schema.Table = m.Table
		}
		out = append(out, schema)
	}
	return out, nil
}
