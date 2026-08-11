package view

import (
	"net/http"

	"github.com/farhapartex/coyote/core/session"
)

type Data map[string]any

func (d Data) Set(key string, value any) Data {
	d[key] = value
	return d
}

func (d Data) SetDefault(key string, value any) Data {
	if _, exists := d[key]; !exists {
		d[key] = value
	}
	return d
}

func (d Data) Merge(other Data) Data {
	for key, value := range other {
		d[key] = value
	}
	return d
}

func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func RedirectPermanent(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func Flash(r *http.Request, kind, message string) {
	if sess := session.FromRequest(r); sess != nil {
		sess.AddFlash(kind, message)
	}
}

func Success(r *http.Request, message string) { Flash(r, "success", message) }

func Error(r *http.Request, message string) { Flash(r, "error", message) }

func Warning(r *http.Request, message string) { Flash(r, "warning", message) }

func Info(r *http.Request, message string) { Flash(r, "info", message) }
