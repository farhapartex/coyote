package admin

import (
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/view"
)

const dashboardRecent = 5

func (a *Admin) dashboard(w http.ResponseWriter, r *http.Request) {
	users := a.app.Auth.Users()

	stats, err := users.Stats(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	recent, err := users.Recent(r.Context(), dashboardRecent)
	if err != nil {
		a.fail(w, r, err)
		return
	}

	a.render(w, r, http.StatusOK, "dashboard.html", view.Data{
		"Nav":             "dashboard",
		"UserCount":       stats.Total,
		"SuperadminCount": stats.Superadmins,
		"StaffCount":      stats.Staff,
		"SessionCount":    a.sessionCount(r.Context()),
		"Uptime":          time.Since(a.app.Started).Round(time.Second).String(),
		"Recent":          recent,
	})
}
