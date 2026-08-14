package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/model"
)

type binding struct {
	Record model.Record
	Errors map[string]string
}

func (b binding) valid() bool { return len(b.Errors) == 0 }

func bindForm(r *http.Request, schema *model.Schema, creating bool) binding {
	out := binding{Record: model.Record{}, Errors: map[string]string{}}

	for _, f := range schema.FormFields() {
		if f.PrimaryKey && !creating {
			continue
		}
		raw := strings.TrimSpace(r.PostForm.Get(f.Column))

		if f.Kind == model.KindBool {
			out.Record[f.Column] = raw != ""
			continue
		}
		if raw == "" {
			if f.Sensitive && !creating {
				continue
			}
			if f.Required {
				out.Errors[f.Column] = f.Label + " is required"
				continue
			}
			if f.PrimaryKey {
				continue
			}
			out.Record[f.Column] = zeroFor(f)
			continue
		}
		value, err := coerce(f, raw)
		if err != "" {
			out.Errors[f.Column] = err
			continue
		}
		out.Record[f.Column] = value
	}
	return out
}

func coerce(f model.Field, raw string) (any, string) {
	switch f.Kind {
	case model.KindInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, f.Label + " must be a whole number"
		}
		return n, ""
	case model.KindFloat:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, f.Label + " must be a number"
		}
		return n, ""
	case model.KindTime:
		stamp, err := time.Parse(timeLayout, raw)
		if err != nil {
			return nil, f.Label + " must be a date and time"
		}
		return stamp, ""
	default:
		if f.Size > 0 && len(raw) > f.Size {
			return nil, f.Label + " must be " + strconv.Itoa(f.Size) + " characters or fewer"
		}
		return raw, ""
	}
}

func zeroFor(f model.Field) any {
	switch f.Kind {
	case model.KindInt:
		return int64(0)
	case model.KindFloat:
		return float64(0)
	case model.KindBool:
		return false
	case model.KindTime:
		return nil
	default:
		return ""
	}
}
