package form

import (
	"fmt"
	"sort"
	"strings"
)

type Problems map[string][]string

func (p Problems) Add(field, message string) {
	p[field] = append(p[field], message)
}

func (p Problems) Any() bool { return len(p) > 0 }

func (p Problems) Has(field string) bool { return len(p[field]) > 0 }

func (p Problems) First(field string) string {
	if len(p[field]) == 0 {
		return ""
	}
	return p[field][0]
}

func (p Problems) Fields() []string {
	out := make([]string, 0, len(p))
	for field := range p {
		out = append(out, field)
	}
	sort.Strings(out)
	return out
}

func (p Problems) Error() string {
	parts := make([]string, 0, len(p))
	for _, field := range p.Fields() {
		parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(p[field], ", ")))
	}
	return "coyote/form: " + strings.Join(parts, "; ")
}
