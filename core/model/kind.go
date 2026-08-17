package model

import (
	"reflect"
	"time"
)

type Kind string

const (
	KindString Kind = "string"
	KindText   Kind = "text"
	KindInt    Kind = "int"
	KindFloat  Kind = "float"
	KindBool   Kind = "bool"
	KindTime   Kind = "time"
	KindBytes  Kind = "bytes"
	KindFile   Kind = "file"
)

var timeType = reflect.TypeOf(time.Time{})

func KindOf(t reflect.Type) (Kind, bool) {
	nullable := false
	for t != nil && t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}
	if t == nil {
		return KindString, nullable
	}
	if t == timeType {
		return KindTime, nullable
	}
	switch t.Kind() {
	case reflect.String:
		return KindString, nullable
	case reflect.Bool:
		return KindBool, nullable
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return KindInt, nullable
	case reflect.Float32, reflect.Float64:
		return KindFloat, nullable
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return KindBytes, nullable
		}
	}
	return KindString, nullable
}
