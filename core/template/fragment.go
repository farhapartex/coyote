package template

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"reflect"
	"strings"
	"time"
)

const fragmentPrefix = "fragment:"

var guardedKeys = []string{"CSRFToken", "Nonce"}

func (e *Engine) Fragment(name string, ttl int, part any, data any) (htmltemplate.HTML, error) {
	if e.fragments == nil {
		return e.renderFragment(name, data)
	}

	key := fragmentKey(name, part, data)
	ctx := context.Background()

	if raw, found, err := e.fragments.Get(ctx, key); err == nil && found {
		return htmltemplate.HTML(raw), nil
	}

	rendered, err := e.renderFragment(name, data)
	if err != nil {
		return "", err
	}
	if leaked := leakedSecret(string(rendered), data); leaked != "" {
		e.report(fmt.Errorf("coyote/render: fragment %q was not cached because it contains the request's %s", name, leaked))
		return rendered, nil
	}
	if err := e.fragments.Set(ctx, key, []byte(rendered), time.Duration(ttl)*time.Second); err != nil {
		e.report(fmt.Errorf("coyote/render: caching fragment %q: %w", name, err))
	}
	return rendered, nil
}

func (e *Engine) renderFragment(name string, data any) (htmltemplate.HTML, error) {
	set, err := e.partialSet()
	if err != nil {
		return "", err
	}
	if set.Lookup(name) == nil {
		return "", fmt.Errorf("coyote/render: no fragment named %q; it must be one of Templates.Shared", name)
	}

	buffer := &bytes.Buffer{}
	if err := set.ExecuteTemplate(buffer, name, data); err != nil {
		return "", fmt.Errorf("coyote/render: executing fragment %q: %w", name, err)
	}
	return htmltemplate.HTML(buffer.String()), nil
}

func (e *Engine) partialSet() (*htmltemplate.Template, error) {
	e.partialMu.Lock()
	defer e.partialMu.Unlock()

	if e.partials != nil && !e.reload {
		return e.partials, nil
	}
	if e.fsys == nil {
		return nil, fmt.Errorf("coyote/render: no template filesystem configured")
	}

	set := htmltemplate.New("fragments").Funcs(e.funcs)
	for _, pattern := range e.shared {
		matches, err := fs.Glob(e.fsys, pattern)
		if err != nil {
			return nil, fmt.Errorf("coyote/render: glob %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			continue
		}
		set, err = set.ParseFS(e.fsys, matches...)
		if err != nil {
			return nil, fmt.Errorf("coyote/render: parsing %q: %w", pattern, err)
		}
	}
	e.partials = set
	return set, nil
}

func (e *Engine) report(err error) {
	if e.onError != nil {
		e.onError(err)
	}
}

func fragmentKey(name string, part any, data any) string {
	key := fragmentPrefix + name + "|" + fmt.Sprint(part)
	if tag := localeTag(data); tag != "" {
		key += "#" + tag
	}
	return key
}

type tagged interface {
	Tag() string
}

func localeTag(data any) string {
	if holder, ok := lookupValue(data, "Locale").(tagged); ok {
		return holder.Tag()
	}
	return ""
}

func leakedSecret(rendered string, data any) string {
	for _, key := range guardedKeys {
		value := lookupString(data, key)
		if value != "" && strings.Contains(rendered, value) {
			return key
		}
	}
	return ""
}

type mintedSecret interface {
	Minted() string
}

func lookupValue(data any, key string) any {
	holder := reflect.ValueOf(data)
	if !holder.IsValid() || holder.Kind() != reflect.Map {
		return nil
	}
	if holder.Type().Key().Kind() != reflect.String {
		return nil
	}
	found := holder.MapIndex(reflect.ValueOf(key).Convert(holder.Type().Key()))
	if !found.IsValid() {
		return nil
	}
	return found.Interface()
}

func lookupString(data any, key string) string {
	switch value := lookupValue(data, key).(type) {
	case string:
		return value
	case mintedSecret:
		return value.Minted()
	}
	return ""
}
