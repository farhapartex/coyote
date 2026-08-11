package view

import "net/http"

func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func RedirectPermanent(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}
