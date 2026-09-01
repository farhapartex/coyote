package form

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/model"
)

const RecordTimeLayout = "2006-01-02T15:04"

type Bound struct {
	Record   model.Record
	Problems Problems
}

func (b Bound) Valid() bool { return !b.Problems.Any() }

func (b Bound) Errors() map[string]string {
	out := make(map[string]string, len(b.Problems))
	for field := range b.Problems {
		out[field] = b.Problems.First(field)
	}
	return out
}

func Record(values url.Values, schema *model.Schema, creating bool) Bound {
	return RecordOf(values, schema, schema.FormFields(), creating)
}

func RecordOf(values url.Values, schema *model.Schema, fields []model.Field, creating bool) Bound {
	out := Bound{Record: model.Record{}, Problems: Problems{}}

	for _, f := range fields {
		if f.Kind == model.KindFile {
			continue
		}
		submitted, present := values[f.Column]
		if !present {
			if f.Sensitive && !creating {
				continue
			}
			if !f.Nullable {
				out.Problems.Add(f.Column, f.Label+" was not submitted")
			}
			continue
		}

		if f.Kind == model.KindBool {
			out.Record[f.Column] = anyTruthy(submitted)
			continue
		}
		raw := strings.TrimSpace(values.Get(f.Column))
		if raw == "" {
			if f.Sensitive && !creating {
				continue
			}
			if f.Required {
				out.Problems.Add(f.Column, f.Label+" is required")
				continue
			}
			out.Record[f.Column] = zeroFor(f)
			continue
		}
		value, problem := coerce(f, raw)
		if problem != "" {
			out.Problems.Add(f.Column, problem)
			continue
		}
		out.Record[f.Column] = value
	}
	return out
}

func anyTruthy(submitted []string) bool {
	for _, value := range submitted {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
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
		stamp, err := time.Parse(RecordTimeLayout, raw)
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
