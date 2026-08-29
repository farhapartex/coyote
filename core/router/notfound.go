package router

import (
	"bytes"
	"net/http"
)

func (r *Router) SetNotFound(handler http.Handler) {
	*r.notFound = handler
}

func (r *Router) NotFound() http.Handler {
	return *r.notFound
}

func (r *Router) serveWithFallback(w http.ResponseWriter, req *http.Request, fallback http.Handler) {
	if _, pattern := r.mux.Handler(req); pattern != "" {
		r.mux.ServeHTTP(w, req)
		return
	}

	held := &heldResponse{header: http.Header{}, status: http.StatusOK}
	r.mux.ServeHTTP(held, req)

	if held.status == http.StatusNotFound {
		fallback.ServeHTTP(w, req)
		return
	}
	held.flushTo(w)
}

type heldResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
	wrote  bool
}

func (h *heldResponse) Header() http.Header {
	return h.header
}

func (h *heldResponse) WriteHeader(status int) {
	if h.wrote {
		return
	}
	h.status = status
	h.wrote = true
}

func (h *heldResponse) Write(p []byte) (int, error) {
	h.wrote = true
	return h.body.Write(p)
}

func (h *heldResponse) flushTo(w http.ResponseWriter) {
	for key, values := range h.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(h.status)
	if h.body.Len() > 0 {
		w.Write(h.body.Bytes())
	}
}
