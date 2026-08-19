package admin

import (
	"strconv"
	"time"

	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/model"
)

type formField struct {
	Column   string
	Label    string
	Type     string
	Value    string
	Checked  bool
	Required bool
	ReadOnly bool
	Multi    bool
	Error    string
	Step     string
	MaxLen   int
	Accept   string
	URL      string
	Options  []option
	Relation bool
}

func inputType(f model.Field) string {
	switch {
	case f.Kind == model.KindFile:
		return "file"
	case f.Sensitive:
		return "password"
	case f.Column == "email":
		return "email"
	case f.Kind == model.KindBool:
		return "checkbox"
	case f.Kind == model.KindInt, f.Kind == model.KindFloat:
		return "number"
	case f.Kind == model.KindTime:
		return "datetime-local"
	default:
		return "text"
	}
}

func buildForm(schema *model.Schema, record model.Record, errs map[string]string, readOnly bool) []formField {
	display := schema.DisplayFields(record != nil)
	fields := make([]formField, 0, len(display))
	for _, f := range display {
		entry := formField{
			Column:   f.Column,
			Label:    f.Label,
			Type:     inputType(f),
			Required: f.Required && !f.PrimaryKey,
			ReadOnly: readOnly || f.PrimaryKey,
			Multi:    f.Kind == model.KindText,
			Accept:   f.Accept,
			Error:    errs[f.Column],
			MaxLen:   f.Size,
		}
		if f.Kind == model.KindFloat {
			entry.Step = "any"
		}
		if record != nil {
			entry.Checked = record.Bool(f.Column)
			entry.Value = formValue(f, record)
		}
		fields = append(fields, entry)
	}
	return fields
}

func formValue(f model.Field, record model.Record) string {
	if f.Sensitive {
		return ""
	}
	raw := record.Get(f.Column)
	if raw == nil {
		return ""
	}
	if f.Kind == model.KindTime {
		if stamp, ok := raw.(time.Time); ok {
			if stamp.IsZero() {
				return ""
			}
			return stamp.Format(form.RecordTimeLayout)
		}
	}
	if f.Kind == model.KindBool {
		return strconv.FormatBool(record.Bool(f.Column))
	}
	return record.String(f.Column)
}
