package admin

import (
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) userList(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	all := a.app.Auth.Users().All()
	matched := make([]*auth.User, 0, len(all))
	for _, u := range all {
		if query == "" ||
			strings.Contains(strings.ToLower(u.Username), query) ||
			strings.Contains(strings.ToLower(u.Email), query) ||
			strings.Contains(strings.ToLower(u.FullName()), query) {
			matched = append(matched, u)
		}
	}
	a.render(w, r, http.StatusOK, "users.html", view.Data{
		"Nav":   "users",
		"Users": matched,
		"Query": r.URL.Query().Get("q"),
		"Total": len(all),
	})
}

func (a *Admin) roleChoices(userID string) ([]roleChoice, error) {
	store := a.app.Auth.Permissions()
	if store == nil {
		return nil, nil
	}
	roles, err := store.AllRoles()
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	if userID != "" {
		assigned, err := store.RolesForUser(userID)
		if err != nil {
			return nil, err
		}
		for _, roleID := range assigned {
			held[roleID] = true
		}
	}
	out := make([]roleChoice, 0, len(roles))
	for _, role := range roles {
		out = append(out, roleChoice{ID: role.ID, Name: role.Name, Held: held[role.ID]})
	}
	return out, nil
}

func (a *Admin) saveUserRoles(r *http.Request, userID string) error {
	store := a.app.Auth.Permissions()
	if store == nil {
		return nil
	}
	return store.SetUserRoles(userID, r.PostForm["roles"])
}

func (a *Admin) userForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data := view.Data{"Nav": "users", "IsNew": true, "Form": &auth.User{IsActive: true}}
	choices, err := a.roleChoices(id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	data["Roles"] = choices
	data["AllowPasswordChange"] = a.app.Auth.AllowsPasswordChange()
	if id != "" {
		user, err := a.app.Auth.Users().ByID(id)
		if err != nil {
			a.notFound(w, r)
			return
		}
		data["IsNew"] = false
		data["Form"] = user
	}
	a.render(w, r, http.StatusOK, "user_form.html", data)
}

func (a *Admin) userCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	form := &auth.User{
		Username:     strings.TrimSpace(r.PostForm.Get("username")),
		Email:        strings.TrimSpace(r.PostForm.Get("email")),
		FirstName:    strings.TrimSpace(r.PostForm.Get("first_name")),
		LastName:     strings.TrimSpace(r.PostForm.Get("last_name")),
		IsActive:     r.PostForm.Get("is_active") != "",
		IsStaff:      r.PostForm.Get("is_staff") != "",
		IsSuperadmin: r.PostForm.Get("is_superadmin") != "",
	}
	password := r.PostForm.Get("password")

	user, err := a.app.Auth.CreateUser(auth.NewUser{
		Username:     form.Username,
		Email:        form.Email,
		FirstName:    form.FirstName,
		LastName:     form.LastName,
		Password:     password,
		IsStaff:      form.IsStaff,
		IsSuperadmin: form.IsSuperadmin,
	})
	if err != nil {
		a.render(w, r, http.StatusBadRequest, "user_form.html", view.Data{
			"Nav": "users", "IsNew": true, "Form": form, "Error": humanize(r.Context(), err),
		})
		return
	}
	if !form.IsActive {
		user.IsActive = false
		if err := a.app.Auth.Users().Update(user); err != nil {
			a.render(w, r, http.StatusBadRequest, "user_form.html", view.Data{
				"Nav": "users", "IsNew": true, "Form": form, "Error": humanize(r.Context(), err),
			})
			return
		}
	}
	if err := a.saveUserRoles(r, user.ID); err != nil {
		a.fail(w, r, err)
		return
	}
	view.Flash(r, "success", i18n.Tf(r.Context(), "User %s created.", user.Username))
	view.Redirect(w, r, a.prefix+"/users")
}

func (a *Admin) userUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	user, err := a.app.Auth.Users().ByID(id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	me := a.currentUser(r)
	isSelf := me != nil && me.ID == user.ID

	user.Username = strings.TrimSpace(r.PostForm.Get("username"))
	user.Email = strings.TrimSpace(r.PostForm.Get("email"))
	user.FirstName = strings.TrimSpace(r.PostForm.Get("first_name"))
	user.LastName = strings.TrimSpace(r.PostForm.Get("last_name"))
	if !isSelf {
		user.IsActive = r.PostForm.Get("is_active") != ""
		user.IsSuperadmin = r.PostForm.Get("is_superadmin") != ""
		user.IsStaff = r.PostForm.Get("is_staff") != "" || user.IsSuperadmin
	}

	fail := func(err error) {
		a.render(w, r, http.StatusBadRequest, "user_form.html", view.Data{
			"Nav": "users", "IsNew": false, "Form": user, "Error": humanize(r.Context(), err),
		})
	}

	if password := r.PostForm.Get("password"); password != "" && a.app.Auth.AllowsPasswordChange() {
		if err := a.app.Auth.ValidatePasswordFor(password, user); err != nil {
			fail(err)
			return
		}
		hash, err := a.app.Auth.HashPassword(password)
		if err != nil {
			fail(err)
			return
		}
		user.Password = hash
		a.revokeUserSessions(user.ID)
	}

	if err := a.app.Auth.Users().Update(user); err != nil {
		fail(err)
		return
	}
	if !isSelf {
		if err := a.saveUserRoles(r, user.ID); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	view.Flash(r, "success", i18n.Tf(r.Context(), "User %s updated.", user.Username))
	view.Redirect(w, r, a.prefix+"/users")
}

func (a *Admin) userDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	me := a.currentUser(r)
	if me != nil && me.ID == id {
		view.Flash(r, "error", i18n.T(r.Context(), "You cannot delete your own account."))
		view.Redirect(w, r, a.prefix+"/users")
		return
	}
	if err := a.app.Auth.Users().Delete(id); err != nil {
		view.Flash(r, "error", humanize(r.Context(), err))
		view.Redirect(w, r, a.prefix+"/users")
		return
	}
	a.revokeUserSessions(id)
	view.Flash(r, "success", i18n.T(r.Context(), "User deleted."))
	view.Redirect(w, r, a.prefix+"/users")
}
