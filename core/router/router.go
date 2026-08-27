package router

import "net/http"

type Middleware func(http.Handler) http.Handler

type Router struct {
	mux      *http.ServeMux
	prefix   string
	chain    []Middleware
	routes   *[]*Route
	notFound *http.Handler
}

func New() *Router {
	routes := make([]*Route, 0, 16)
	var fallback http.Handler
	return &Router{mux: http.NewServeMux(), routes: &routes, notFound: &fallback}
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
	if fallback := *r.notFound; fallback != nil {
		r.serveWithFallback(w, req, fallback)
		return
	}
	r.mux.ServeHTTP(w, req)
}
