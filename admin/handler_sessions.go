package admin

import (
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/view"
	"github.com/farhapartex/coyote/lib/text"
)

type sessionRow struct {
	ID       string
	Short    string
	Username string
	Created  time.Time
	Expires  time.Time
	IsSelf   bool
}

func (a *Admin) sessionList(w http.ResponseWriter, r *http.Request) {
	store, ok := a.sessionStore()
	if !ok {
		a.render(w, r, http.StatusOK, "sessions.html", view.Data{
			"Nav":         "sessions",
			"Unsupported": true,
		})
		return
	}
	current := session.FromRequest(r)
	rows := []sessionRow{}
	for _, s := range store.All() {
		username := "anonymous"
		if id := s.UserID(); id != "" {
			if u, err := a.app.Auth.Users().ByID(id); err == nil {
				username = u.Username
			} else {
				username = "unknown"
			}
		}
		rows = append(rows, sessionRow{
			ID:       s.ID(),
			Short:    text.Truncate(s.ID(), 16),
			Username: username,
			Created:  s.CreatedAt(),
			Expires:  s.ExpiresAt(),
			IsSelf:   current != nil && current.ID() == s.ID(),
		})
	}
	a.render(w, r, http.StatusOK, "sessions.html", view.Data{
		"Nav":      "sessions",
		"Sessions": rows,
	})
}

func (a *Admin) sessionRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	current := session.FromRequest(r)
	if current != nil && current.ID() == id {
		view.Flash(r, "error", "Use log out to end your own session.")
		view.Redirect(w, r, a.prefix+"/sessions")
		return
	}
	if store, ok := a.sessionStore(); ok {
		_ = store.Delete(id)
		view.Flash(r, "success", "Session revoked.")
	} else {
		view.Flash(r, "error", "The configured session store cannot revoke sessions.")
	}
	view.Redirect(w, r, a.prefix+"/sessions")
}
