package router

import (
	"net/http"
	"strings"
)

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
