package admin

import "github.com/farhapartex/coyote/core/app"

func (a *Admin) routes(group *app.Router, loginURL string) {
	group.Get("/favicon.png", a.favicon)
	group.Get("/login", a.loginForm)
	group.Post("/login", a.loginSubmit)

	guarded := group.Group("", a.app.Auth.RequireSuperadmin(loginURL))
	guarded.Post("/logout", a.logout)
	guarded.Get("/{$}", a.dashboard)
	guarded.Get("/users", a.userList)
	guarded.Get("/users/new", a.userForm)
	guarded.Post("/users/new", a.userCreate)
	guarded.Get("/users/{id}", a.userForm)
	guarded.Post("/users/{id}", a.userUpdate)
	guarded.Post("/users/{id}/delete", a.userDelete)
	guarded.Get("/sessions", a.sessionList)
	guarded.Post("/sessions/{id}/revoke", a.sessionRevoke)

	a.guarded = guarded
}
