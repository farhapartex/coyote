package model

import "reflect"

type Fileish interface {
	IsFile() bool
}

var fileType = reflect.TypeOf((*Fileish)(nil)).Elem()

func looksLikeFile(t reflect.Type) bool {
	if t == nil {
		return false
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Implements(fileType) || reflect.PointerTo(t).Implements(fileType)
}
