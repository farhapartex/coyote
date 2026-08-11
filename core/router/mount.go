package router

import (
	"io/fs"
	"net/http"
	"strings"
)

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

func (r *Router) Static(prefix string, fsys fs.FS) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	full := joinPath(r.prefix, prefix)
	r.mux.Handle("GET "+full, http.StripPrefix(full, http.FileServerFS(fsys)))
	*r.routes = append(*r.routes, Route{Method: http.MethodGet, Pattern: full + "*"})
}
