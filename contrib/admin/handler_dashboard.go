package admin

import (
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) dashboard(w http.ResponseWriter, r *http.Request) {
	users := a.app.Auth.Users().All()
	superadmins, staff := 0, 0
	for _, u := range users {
		if u.IsSuperadmin {
			superadmins++
		}
		if u.IsStaff && !u.IsSuperadmin {
			staff++
		}
	}
	a.render(w, r, http.StatusOK, "dashboard.html", view.Data{
		"Nav":             "dashboard",
		"UserCount":       len(users),
		"SuperadminCount": superadmins,
		"StaffCount":      staff,
		"SessionCount":    a.sessionCount(),
		"Uptime":          time.Since(a.app.Started).Round(time.Second).String(),
		"Recent":          recentUsers(users, 5),
	})
}

func recentUsers(users []*auth.User, n int) []*auth.User {
	sorted := append([]*auth.User{}, users...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].CreatedAt.After(sorted[j-1].CreatedAt); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}
