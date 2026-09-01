package admin

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/view"
)

type permissionGroup struct {
	Resource string
	Items    []permissionChoice
}

type permissionChoice struct {
	ID       string
	Action   string
	Codename string
	Granted  bool
}

func (a *Admin) roleList(w http.ResponseWriter, r *http.Request) {
	store, ok := a.permissionStore(w, r)
	if !ok {
		return
	}
	roles, err := store.AllRoles(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	counts := map[string]int{}
	for _, role := range roles {
		granted, err := store.RolePermissions(r.Context(), role.ID)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		counts[role.ID] = len(granted)
	}
	a.render(w, r, http.StatusOK, "roles.html", view.Data{
		"Nav":    "roles",
		"Roles":  roles,
		"Counts": counts,
	})
}

func (a *Admin) roleForm(w http.ResponseWriter, r *http.Request) {
	store, ok := a.permissionStore(w, r)
	if !ok {
		return
	}

	role := &auth.Role{}
	granted := []string{}
	isNew := true

	if id := r.PathValue("id"); id != "" {
		found, err := store.RoleByID(r.Context(), id)
		if err != nil {
			a.notFound(w, r)
			return
		}
		role, isNew = found, false
		if granted, err = store.RolePermissions(r.Context(), id); err != nil {
			a.fail(w, r, err)
			return
		}
	}

	groups, err := a.permissionGroups(r.Context(), store, granted)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "role_form.html", view.Data{
		"Nav": "roles", "IsNew": isNew, "Form": role, "Groups": groups,
	})
}

func (a *Admin) roleSave(w http.ResponseWriter, r *http.Request) {
	store, ok := a.permissionStore(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}

	id := r.PathValue("id")
	role := &auth.Role{
		ID:          id,
		Name:        strings.TrimSpace(r.PostForm.Get("name")),
		Description: strings.TrimSpace(r.PostForm.Get("description")),
	}
	chosen := r.PostForm["permissions"]

	fail := func(message string) {
		groups, err := a.permissionGroups(r.Context(), store, chosen)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		a.render(w, r, http.StatusBadRequest, "role_form.html", view.Data{
			"Nav": "roles", "IsNew": id == "", "Form": role, "Groups": groups, "Error": message,
		})
	}

	if role.Name == "" {
		fail(i18n.T(r.Context(), "A role needs a name."))
		return
	}

	if id == "" {
		if err := store.CreateRole(r.Context(), role); err != nil {
			fail(humanize(r.Context(), err))
			return
		}
	} else {
		existing, err := store.RoleByID(r.Context(), id)
		if err != nil {
			a.notFound(w, r)
			return
		}
		existing.Name = role.Name
		existing.Description = role.Description
		if err := store.UpdateRole(r.Context(), existing); err != nil {
			fail(humanize(r.Context(), err))
			return
		}
		role = existing
	}

	if err := store.SetRolePermissions(r.Context(), role.ID, chosen); err != nil {
		a.fail(w, r, err)
		return
	}
	view.Success(r, i18n.Tf(r.Context(), "Saved %s.", role.Name))
	view.Redirect(w, r, a.prefix+"/roles")
}

func (a *Admin) roleDelete(w http.ResponseWriter, r *http.Request) {
	store, ok := a.permissionStore(w, r)
	if !ok {
		return
	}
	if err := store.DeleteRole(r.Context(), r.PathValue("id")); err != nil {
		a.fail(w, r, err)
		return
	}
	view.Success(r, i18n.T(r.Context(), "Role deleted."))
	view.Redirect(w, r, a.prefix+"/roles")
}

func (a *Admin) permissionGroups(ctx context.Context, store auth.PermissionStore, granted []string) ([]permissionGroup, error) {
	all, err := store.AllPermissions(ctx)
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for _, id := range granted {
		held[id] = true
	}

	byResource := map[string][]permissionChoice{}
	for _, permission := range all {
		byResource[permission.Resource] = append(byResource[permission.Resource], permissionChoice{
			ID:       permission.ID,
			Action:   permission.Action,
			Codename: permission.Codename(),
			Granted:  held[permission.ID],
		})
	}

	groups := make([]permissionGroup, 0, len(byResource))
	for resource, items := range byResource {
		sort.Slice(items, func(i, j int) bool { return items[i].Action < items[j].Action })
		groups = append(groups, permissionGroup{Resource: resource, Items: items})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Resource < groups[j].Resource })
	return groups, nil
}

func (a *Admin) permissionStore(w http.ResponseWriter, r *http.Request) (auth.PermissionStore, bool) {
	store := a.app.Auth.Permissions()
	if store == nil {
		a.render(w, r, http.StatusNotFound, "notfound.html", view.Data{
			"Message": i18n.T(r.Context(), "Permissions are turned off for this project."),
		})
		return nil, false
	}
	return store, true
}

type roleChoice struct {
	ID   string
	Name string
	Held bool
}
