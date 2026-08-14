package router

import "net/http"

type Middleware func(http.Handler) http.Handler

type Router struct {
	mux    *http.ServeMux
	prefix string
	chain  []Middleware
	routes *[]*Route
}

func New() *Router {
	routes := make([]*Route, 0, 16)
	return &Router{mux: http.NewServeMux(), routes: &routes}
}

func (r *Router) Handle(method, pattern string, h http.Handler, mw ...Middleware) *Route {
	full := joinPath(r.prefix, pattern)
	wrapped := chain(h, append(append([]Middleware{}, r.chain...), mw...)...)
	key := full
	if method != "" {
		key = method + " " + full
	}
	r.mux.Handle(key, wrapped)

	route := &Route{Method: methodLabel(method), Pattern: full, routes: r.routes}
	*r.routes = append(*r.routes, route)
	return route
}

func (r *Router) HandleFunc(method, pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle(method, pattern, fn, mw...)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
