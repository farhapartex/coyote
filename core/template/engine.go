package template

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"net/http"
	"path"
	"sync"
)

type Options struct {
	FS     fs.FS
	Layout string
	Shared []string
	Funcs  htmltemplate.FuncMap
	Reload bool
}

type Engine struct {
	fsys   fs.FS
	layout string
	shared []string
	funcs  htmltemplate.FuncMap
	reload bool
	mu     sync.RWMutex
	cache  map[string]*htmltemplate.Template
}

func New(opts Options) *Engine {
	if opts.Layout == "" {
		opts.Layout = "base.html"
	}
	if opts.Shared == nil {
		opts.Shared = []string{"layouts/*.html", "partials/*.html"}
	}
	funcs := defaultFuncs()
	for name, fn := range opts.Funcs {
		funcs[name] = fn
	}
	return &Engine{
		fsys:   opts.FS,
		layout: path.Base(opts.Layout),
		shared: opts.Shared,
		funcs:  funcs,
		reload: opts.Reload,
		cache:  make(map[string]*htmltemplate.Template),
	}
}

func (e *Engine) Render(w http.ResponseWriter, status int, page string, data any) error {
	buf, err := e.RenderToBuffer(page, data)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err = buf.WriteTo(w)
	return err
}

func (e *Engine) RenderToBuffer(page string, data any) (*bytes.Buffer, error) {
	tmpl, err := e.lookup(page)
	if err != nil {
		return nil, err
	}
	name := e.layout
	if tmpl.Lookup(name) == nil {
		name = path.Base(page)
	}
	buf := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		return nil, fmt.Errorf("coyote/render: executing %q: %w", page, err)
	}
	return buf, nil
}

func (e *Engine) Has(page string) bool {
	if e.fsys == nil {
		return false
	}
	f, err := e.fsys.Open(page)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
