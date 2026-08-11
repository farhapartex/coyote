package template

import (
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"path"
)

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
