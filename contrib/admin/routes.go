package admin

import "github.com/farhapartex/coyote/core/app"

func (a *Admin) routes(group *app.Router, loginURL string) {
	group.Get("/favicon.png", a.favicon)
	group.Get("/login", a.loginForm)
	group.Post("/login", a.loginSubmit)

	guarded := group.Group("", a.app.Auth.RequireStaff(loginURL))
	guarded.Post("/logout", a.logout)
	guarded.Get("/{$}", a.dashboard)

	privileged := group.Group("", a.app.Auth.RequireSuperadmin(loginURL))
	privileged.Get("/users", a.userList)
	privileged.Get("/users/new", a.userForm)
	privileged.Post("/users/new", a.userCreate)
	privileged.Get("/users/{id}", a.userForm)
	privileged.Post("/users/{id}", a.userUpdate)
	privileged.Post("/users/{id}/delete", a.userDelete)
	privileged.Post("/users/bulk", a.userBulk)
	privileged.Get("/roles", a.roleList)
	privileged.Get("/roles/new", a.roleForm)
	privileged.Post("/roles/new", a.roleSave)
	privileged.Get("/roles/{id}", a.roleForm)
	privileged.Post("/roles/{id}", a.roleSave)
	privileged.Post("/roles/{id}/delete", a.roleDelete)
	privileged.Post("/roles/bulk", a.roleBulk)
	privileged.Get("/jobs", a.jobList)
	privileged.Post("/jobs/{id}/retry", a.jobRetry)
	privileged.Post("/jobs/bulk", a.jobBulk)
	privileged.Get("/sessions", a.sessionList)
	privileged.Post("/sessions/{id}/revoke", a.sessionRevoke)
	privileged.Post("/sessions/bulk", a.sessionBulk)

	a.guarded = guarded
}
