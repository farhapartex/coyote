package view

import (
	"net/http"
	"strings"
)

func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func RedirectPermanent(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func SafeNext(next, fallback string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return fallback
	}
	return next
}
