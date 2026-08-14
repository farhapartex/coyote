package app

import htmltemplate "html/template"

func templateFuncs(s Settings, a *App) htmltemplate.FuncMap {
	funcs := htmltemplate.FuncMap{
		"url":    a.Reverse,
		"scheme": s.Scheme,
		"base":   s.BaseURL,
	}
	for name, fn := range s.Templates.Funcs {
		funcs[name] = fn
	}
	return funcs
}
