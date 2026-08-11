package router

import (
	"io/fs"
	"net/http"
	"sort"
	"strings"
)

type Middleware func(http.Handler) http.Handler

type Route struct {
	Method  string
	Pattern string
	Name    string
}

type Router struct {
	mux    *http.ServeMux
	prefix string
	chain  []Middleware
	routes *[]Route
}

func New() *Router {
	routes := make([]Route, 0, 16)
	return &Router{mux: http.NewServeMux(), routes: &routes}
}

func (r *Router) Group(prefix string, mw ...Middleware) *Router {
	return &Router{
		mux:    r.mux,
		prefix: joinPath(r.prefix, prefix),
		chain:  append(append([]Middleware{}, r.chain...), mw...),
		routes: r.routes,
	}
}

func (r *Router) Use(mw ...Middleware) {
	r.chain = append(r.chain, mw...)
}

func (r *Router) Handle(method, pattern string, h http.Handler, mw ...Middleware) {
	full := joinPath(r.prefix, pattern)
	wrapped := chain(h, append(append([]Middleware{}, r.chain...), mw...)...)
	key := full
	if method != "" {
		key = method + " " + full
	}
	r.mux.Handle(key, wrapped)
	*r.routes = append(*r.routes, Route{Method: methodLabel(method), Pattern: full})
}

func (r *Router) HandleFunc(method, pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle(method, pattern, fn, mw...)
}

func (r *Router) Get(pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodGet, pattern, fn, mw...)
}

func (r *Router) Post(pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodPost, pattern, fn, mw...)
}

func (r *Router) Put(pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodPut, pattern, fn, mw...)
}

func (r *Router) Patch(pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodPatch, pattern, fn, mw...)
}

func (r *Router) Delete(pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodDelete, pattern, fn, mw...)
}

func (r *Router) Any(pattern string, fn http.HandlerFunc, mw ...Middleware) {
	r.Handle("", pattern, fn, mw...)
}

var mountMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodOptions,
}

func (r *Router) Mount(prefix string, h http.Handler, mw ...Middleware) {
	full := joinPath(r.prefix, prefix)
	if !strings.HasSuffix(full, "/") {
		full += "/"
	}
	stripped := http.StripPrefix(strings.TrimSuffix(full, "/"), h)
	wrapped := chain(stripped, append(append([]Middleware{}, r.chain...), mw...)...)
	for _, method := range mountMethods {
		r.mux.Handle(method+" "+full, wrapped)
	}
	*r.routes = append(*r.routes, Route{Method: "ANY", Pattern: full + "*"})
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

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) Static(prefix string, fsys fs.FS) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	full := joinPath(r.prefix, prefix)
	r.mux.Handle("GET "+full, http.StripPrefix(full, http.FileServerFS(fsys)))
	*r.routes = append(*r.routes, Route{Method: http.MethodGet, Pattern: full + "*"})
}

func Chain(h http.Handler, mw ...Middleware) http.Handler {
	return chain(h, mw...)
}

func chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		if mw[i] != nil {
			h = mw[i](h)
		}
	}
	return h
}

func joinPath(prefix, pattern string) string {
	if prefix == "" {
		return pattern
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if pattern == "" || pattern == "/" {
		return prefix + "/"
	}
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}
	return prefix + pattern
}

func methodLabel(method string) string {
	if method == "" {
		return "ANY"
	}
	return method
}
