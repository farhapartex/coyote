package model

import (
	"fmt"
	"time"
)

type Record map[string]any

func (r Record) Get(column string) any { return r[column] }

func (r Record) String(column string) string {
	value, ok := r[column]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		if typed.IsZero() {
			return ""
		}
		return typed.Format("2 Jan 2006 15:04")
	case bool:
		if typed {
			return "yes"
		}
		return "no"
	default:
		return fmt.Sprintf("%v", value)
	}
}

func (r Record) Bool(column string) bool {
	switch typed := r[column].(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case int:
		return typed != 0
	}
	return false
}
