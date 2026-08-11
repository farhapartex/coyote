package template

import (
	htmltemplate "html/template"
	"strings"
)

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
