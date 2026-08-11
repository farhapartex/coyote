package view

import (
	"net/http"

	"github.com/farhapartex/coyote/core/session"
)

func Flash(r *http.Request, kind, message string) {
	if sess := session.FromRequest(r); sess != nil {
		sess.AddFlash(kind, message)
	}
}

func Success(r *http.Request, message string) { Flash(r, "success", message) }

func Error(r *http.Request, message string) { Flash(r, "error", message) }

func Warning(r *http.Request, message string) { Flash(r, "warning", message) }

func Info(r *http.Request, message string) { Flash(r, "info", message) }
