package router

import "net/http"

func (r *Router) Get(pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle(http.MethodGet, pattern, fn, mw...)
}

func (r *Router) Post(pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle(http.MethodPost, pattern, fn, mw...)
}

func (r *Router) Put(pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle(http.MethodPut, pattern, fn, mw...)
}

func (r *Router) Patch(pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle(http.MethodPatch, pattern, fn, mw...)
}

func (r *Router) Delete(pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle(http.MethodDelete, pattern, fn, mw...)
}

func (r *Router) Any(pattern string, fn http.HandlerFunc, mw ...Middleware) *Route {
	return r.Handle("", pattern, fn, mw...)
}
