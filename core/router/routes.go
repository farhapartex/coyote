package router

import "sort"

type Route struct {
	Method  string
	Pattern string
	Name    string
}

func (r *Router) Routes() []Route {
	out := append([]Route{}, *r.routes...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pattern == out[j].Pattern {
			return out[i].Method < out[j].Method
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}
