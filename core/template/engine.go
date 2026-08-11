package template

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
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

func (e *Engine) lookup(page string) (*htmltemplate.Template, error) {
	if e.fsys == nil {
		return nil, fmt.Errorf("coyote/render: no template filesystem configured")
	}
	if !e.reload {
		e.mu.RLock()
		cached, ok := e.cache[page]
		e.mu.RUnlock()
		if ok {
			return cached, nil
		}
	}
	tmpl, err := e.parse(page)
	if err != nil {
		return nil, err
	}
	if !e.reload {
		e.mu.Lock()
		e.cache[page] = tmpl
		e.mu.Unlock()
	}
	return tmpl, nil
}

func (e *Engine) parse(page string) (*htmltemplate.Template, error) {
	tmpl := htmltemplate.New(path.Base(page)).Funcs(e.funcs)
	for _, pattern := range e.shared {
		matches, err := fs.Glob(e.fsys, pattern)
		if err != nil {
			return nil, fmt.Errorf("coyote/render: glob %q: %w", pattern, err)
		}
		matches = exclude(matches, page)
		if len(matches) == 0 {
			continue
		}
		tmpl, err = tmpl.ParseFS(e.fsys, matches...)
		if err != nil {
			return nil, fmt.Errorf("coyote/render: parsing %q: %w", pattern, err)
		}
	}
	tmpl, err := tmpl.ParseFS(e.fsys, page)
	if err != nil {
		return nil, fmt.Errorf("coyote/render: parsing %q: %w", page, err)
	}
	return tmpl, nil
}

func exclude(paths []string, drop string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != drop {
			out = append(out, p)
		}
	}
	return out
}

func defaultFuncs() htmltemplate.FuncMap {
	return htmltemplate.FuncMap{
		"safe":  func(s string) htmltemplate.HTML { return htmltemplate.HTML(s) },
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"hasPrefix": strings.HasPrefix,
		"join":      strings.Join,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"default": func(fallback, value any) any {
			if value == nil || value == "" {
				return fallback
			}
			return value
		},
		"dict": func(pairs ...any) map[string]any {
			out := make(map[string]any, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				key, ok := pairs[i].(string)
				if !ok {
					continue
				}
				out[key] = pairs[i+1]
			}
			return out
		},
	}
}
