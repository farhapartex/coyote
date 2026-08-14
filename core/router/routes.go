package router

import (
	"fmt"
	"sort"
)

type Route struct {
	Method  string
	Pattern string
	Name    string

	routes *[]*Route
}

func (route *Route) Named(name string) *Route {
	if name == "" {
		return route
	}
	if route.routes != nil {
		for _, existing := range *route.routes {
			if existing != route && existing.Name == name {
				panic(fmt.Errorf("%w: %q is already used by %s %s",
					ErrDuplicateName, name, existing.Method, existing.Pattern))
			}
		}
	}
	route.Name = name
	return route
}

func (r *Router) Routes() []Route {
	out := make([]Route, 0, len(*r.routes))
	for _, route := range *r.routes {
		out = append(out, Route{Method: route.Method, Pattern: route.Pattern, Name: route.Name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pattern == out[j].Pattern {
			return out[i].Method < out[j].Method
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}

func (r *Router) Named(name string) (Route, bool) {
	for _, route := range *r.routes {
		if route.Name == name {
			return Route{Method: route.Method, Pattern: route.Pattern, Name: route.Name}, true
		}
	}
	return Route{}, false
}

func (r *Router) record(route *Route) *Route {
	route.routes = r.routes
	*r.routes = append(*r.routes, route)
	return route
}
