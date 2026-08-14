package router

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrDuplicateName = errors.New("coyote/router: duplicate route name")
	ErrUnknownName   = errors.New("coyote/router: no route with that name")
	ErrWrongArity    = errors.New("coyote/router: wrong number of values for route")
)

func (r *Router) Reverse(name string, values ...any) (string, error) {
	route, found := r.Named(name)
	if !found {
		return "", fmt.Errorf("%w: %q", ErrUnknownName, name)
	}
	return route.Fill(values...)
}

func (r *Router) MustReverse(name string, values ...any) string {
	path, err := r.Reverse(name, values...)
	if err != nil {
		panic(err)
	}
	return path
}

func (route Route) Wildcards() []string {
	var out []string
	for _, segment := range strings.Split(route.Pattern, "/") {
		if wildcard, ok := wildcardName(segment); ok {
			out = append(out, wildcard)
		}
	}
	return out
}

func (route Route) Fill(values ...any) (string, error) {
	segments := strings.Split(route.Pattern, "/")
	filled := make([]string, 0, len(segments))
	used := 0

	for _, segment := range segments {
		if segment == "{$}" {
			continue
		}
		name, ok := wildcardName(segment)
		if !ok {
			filled = append(filled, segment)
			continue
		}
		if used >= len(values) {
			return "", fmt.Errorf("%w %q: %s needs a value", ErrWrongArity, route.Name, name)
		}
		text := fmt.Sprintf("%v", values[used])
		used++
		if text == "" {
			return "", fmt.Errorf("%w %q: %s cannot be empty", ErrWrongArity, route.Name, name)
		}
		if strings.HasSuffix(segment, "...}") {
			filled = append(filled, escapePath(text))
			continue
		}
		filled = append(filled, url.PathEscape(text))
	}

	if used != len(values) {
		return "", fmt.Errorf("%w %q: got %d values for %d wildcards",
			ErrWrongArity, route.Name, len(values), used)
	}

	path := strings.Join(filled, "/")
	if path == "" {
		return "/", nil
	}
	return path, nil
}

func wildcardName(segment string) (string, bool) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") || segment == "{$}" {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
	return strings.TrimSuffix(name, "..."), true
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
