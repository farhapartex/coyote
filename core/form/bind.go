package form

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const DefaultMaxFormBody = 10 << 20

var (
	ErrNotAPointer = errors.New("coyote/form: target must be a pointer to a struct")
	timeLayouts    = []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339, "2006-01-02"}
)

func Bind(r *http.Request, target any) (Problems, error) {
	if err := parse(r, DefaultMaxFormBody); err != nil {
		return nil, err
	}
	return Values(r.PostForm, target)
}

func BindQuery(r *http.Request, target any) (Problems, error) {
	return Values(r.URL.Query(), target)
}

func parse(r *http.Request, maxBytes int64) error {
	if r.Body != nil && maxBytes > 0 {
		r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return r.ParseMultipartForm(maxBytes)
	}
	return r.ParseForm()
}

func Values(values map[string][]string, target any) (Problems, error) {
	pointer := reflect.ValueOf(target)
	if pointer.Kind() != reflect.Pointer || pointer.IsNil() {
		return nil, ErrNotAPointer
	}
	structure := pointer.Elem()
	if structure.Kind() != reflect.Struct {
		return nil, ErrNotAPointer
	}

	problems := Problems{}
	kind := structure.Type()

	for i := range kind.NumField() {
		field := kind.Field(i)
		if !field.IsExported() {
			continue
		}
		name := fieldName(field)
		if name == "-" {
			continue
		}

		raw := ""
		if found, ok := values[name]; ok && len(found) > 0 {
			raw = strings.TrimSpace(found[0])
		}

		if err := assign(structure.Field(i), raw, field); err != nil {
			problems.Add(name, err.Error())
			continue
		}
		for _, problem := range check(field, raw) {
			problems.Add(name, problem)
		}
	}

	if problems.Any() {
		return problems, nil
	}
	return nil, nil
}

func fieldName(field reflect.StructField) string {
	if tag := field.Tag.Get("form"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return strings.ToLower(field.Name)
}

func check(field reflect.StructField, value string) []string {
	tag := field.Tag.Get("validate")
	if tag == "" {
		return nil
	}
	out := []string{}
	for _, entry := range strings.Split(tag, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, arg, _ := strings.Cut(entry, "=")
		rule, found := lookup(name)
		if !found {
			continue
		}
		if err := rule(value, arg); err != nil {
			out = append(out, err.Error())
		}
	}
	return out
}

func assign(target reflect.Value, raw string, field reflect.StructField) error {
	if target.Kind() == reflect.Pointer {
		if raw == "" {
			target.SetZero()
			return nil
		}
		created := reflect.New(target.Type().Elem())
		if err := assign(created.Elem(), raw, field); err != nil {
			return err
		}
		target.Set(created)
		return nil
	}

	switch target.Interface().(type) {
	case time.Time:
		if raw == "" {
			target.Set(reflect.ValueOf(time.Time{}))
			return nil
		}
		for _, layout := range timeLayouts {
			if parsed, err := time.Parse(layout, raw); err == nil {
				target.Set(reflect.ValueOf(parsed))
				return nil
			}
		}
		return fmt.Errorf("is not a valid date")
	}

	switch target.Kind() {
	case reflect.String:
		target.SetString(raw)
	case reflect.Bool:
		target.SetBool(truthy(raw))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if raw == "" {
			target.SetInt(0)
			return nil
		}
		number, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a whole number")
		}
		target.SetInt(number)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if raw == "" {
			target.SetUint(0)
			return nil
		}
		number, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a positive whole number")
		}
		target.SetUint(number)
	case reflect.Float32, reflect.Float64:
		if raw == "" {
			target.SetFloat(0)
			return nil
		}
		number, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		target.SetFloat(number)
	}
	return nil
}

func truthy(raw string) bool {
	switch strings.ToLower(raw) {
	case "1", "true", "on", "yes", "checked":
		return true
	}
	return false
}
