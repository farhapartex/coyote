package model

import (
	"reflect"
	"strings"
)

const FileTag = "coyote"

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
	if t.Implements(fileType) || reflect.PointerTo(t).Implements(fileType) {
		return true
	}
	return false
}

type FileOptions struct {
	IsFile bool
	Path   string
	Accept string
}

func ParseFileTag(tag string) FileOptions {
	out := FileOptions{}
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, found := strings.Cut(part, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch {
		case name == "file" && !found:
			out.IsFile = true
		case name == "path":
			out.Path = strings.Trim(value, "/")
			out.IsFile = true
		case name == "accept":
			out.Accept = value
			out.IsFile = true
		}
	}
	return out
}
