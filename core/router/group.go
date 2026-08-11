package router

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
