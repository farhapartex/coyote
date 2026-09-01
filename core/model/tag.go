package model

import "strings"

const Tag = "coyote"

type FieldOptions struct {
	IsFile    bool
	Path      string
	Accept    string
	Sensitive bool
	Public    bool
}

func ParseTag(tag string) FieldOptions {
	out := FieldOptions{}
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
		case name == "sensitive":
			out.Sensitive = true
		case name == "public":
			out.Public = true
		}
	}
	return out
}
