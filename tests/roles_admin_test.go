package tests

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

var roleLinkPattern = regexp.MustCompile(`/admin/roles/([0-9a-f-]{36})`)

func rolePortal(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *client) {
	t.Helper()
	base := func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }
	a := permissionApp(t, append([]func(*settings.Settings){base}, fns...)...)
	if _, err := a.SyncPermissions(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(widgetResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return a, c
}

func (c *client) createRole(t *testing.T, name string, permissionIDs ...string) string {
	t.Helper()
	form := url.Values{
		"csrf_token": {c.token("/admin/roles/new")},
		"name":       {name},
	}
	for _, id := range permissionIDs {
		form.Add("permissions", id)
	}
	if rec := c.do(http.MethodPost, "/admin/roles/new", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("creating a role: %d", rec.Code)
	}
	match := roleLinkPattern.FindStringSubmatch(c.get("/admin/roles").Body.String())
	if match == nil {
		t.Fatal("the new role is not listed")
	}
	return match[1]
}

func permissionID(t *testing.T, a *app.App, codename string) string {
	t.Helper()
	all, err := a.Auth.Permissions().AllPermissions()
	if err != nil {
		t.Fatal(err)
	}
	for _, permission := range all {
		if permission.Codename() == codename {
			return permission.ID
		}
	}
	t.Fatalf("no permission named %q", codename)
	return ""
}

func TestRolesAreListedAndCreatedFromTheAdmin(t *testing.T) {
	a, c := rolePortal(t)

	if body := c.get("/admin/roles").Body.String(); !strings.Contains(body, "No roles yet") {
		t.Error("an empty state should explain what a role is")
	}

	read := permissionID(t, a, "widgets.read")
	roleID := c.createRole(t, "Widget viewer", read)

	body := c.get("/admin/roles").Body.String()
	if !strings.Contains(body, "Widget viewer") {
		t.Error("the new role should be listed")
	}
	if !strings.Contains(body, "1 granted") {
		t.Error("the list should show how many permissions the role carries")
	}

	granted, err := a.Auth.Permissions().RolePermissions(roleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 1 || granted[0] != read {
		t.Errorf("stored permissions = %v, want [%s]", granted, read)
	}
}

func TestRoleFormShowsEveryPermissionGroupedByResource(t *testing.T) {
	_, c := rolePortal(t)

	body := c.get("/admin/roles/new").Body.String()
	for _, want := range []string{"widgets", "users", "widgets.create", "widgets.delete"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form should offer %q", want)
		}
	}
	if strings.Contains(body, "user_roles") {
		t.Error("join tables should not appear as permissions")
	}
}

func TestEditingARoleReplacesItsPermissions(t *testing.T) {
	a, c := rolePortal(t)
	read := permissionID(t, a, "widgets.read")
	update := permissionID(t, a, "widgets.update")
	roleID := c.createRole(t, "Widget viewer", read)

	form := url.Values{
		"csrf_token":  {c.token("/admin/roles/" + roleID)},
		"name":        {"Widget editor"},
		"description": {"can change widgets"},
		"permissions": {read, update},
	}
	if rec := c.do(http.MethodPost, "/admin/roles/"+roleID, form); rec.Code != http.StatusSeeOther {
		t.Fatalf("saving: %d", rec.Code)
	}

	granted, err := a.Auth.Permissions().RolePermissions(roleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 2 {
		t.Errorf("granted = %v, want two", granted)
	}

	if body := c.get("/admin/roles").Body.String(); !strings.Contains(body, "Widget editor") {
		t.Error("the rename should stick")
	}
}

func TestARoleWithoutANameIsRejected(t *testing.T) {
	_, c := rolePortal(t)

	rec := c.do(http.MethodPost, "/admin/roles/new", url.Values{
		"csrf_token": {c.token("/admin/roles/new")},
		"name":       {""},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "needs a name") {
		t.Error("the form should say what went wrong")
	}
}

func TestDeletingARoleRevokesItFromUsers(t *testing.T) {
	a, c := rolePortal(t)
	read := permissionID(t, a, "widgets.read")
	roleID := c.createRole(t, "Widget viewer", read)

	helper, err := a.Auth.CreateUser(auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Auth.Permissions().SetUserRoles(helper.ID, []string{roleID}); err != nil {
		t.Fatal(err)
	}

	codenames, err := a.Auth.GrantedTo(helper.ID)
	if err != nil || len(codenames) != 1 {
		t.Fatalf("codenames = %v, err = %v", codenames, err)
	}

	rec := c.do(http.MethodPost, "/admin/roles/"+roleID+"/delete", url.Values{
		"csrf_token": {c.token("/admin/roles")},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("deleting: %d", rec.Code)
	}

	codenames, err = a.Auth.GrantedTo(helper.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(codenames) != 0 {
		t.Errorf("a deleted role must not leave permissions behind, got %v", codenames)
	}
}

func TestRolesAreAssignedFromTheUserForm(t *testing.T) {
	a, c := rolePortal(t)
	read := permissionID(t, a, "widgets.read")
	roleID := c.createRole(t, "Widget viewer", read)

	form := url.Values{
		"csrf_token": {c.token("/admin/users/new")},
		"username":   {"helper"},
		"password":   {"unrelated-and-long"},
		"is_active":  {"1"},
		"is_staff":   {"1"},
		"roles":      {roleID},
	}
	if rec := c.do(http.MethodPost, "/admin/users/new", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("creating a user: %d", rec.Code)
	}

	helper, err := a.Auth.Users().ByUsername("helper")
	if err != nil {
		t.Fatal(err)
	}
	codenames, err := a.Auth.GrantedTo(helper.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(codenames) != 1 || codenames[0] != "widgets.read" {
		t.Errorf("granted = %v, want [widgets.read]", codenames)
	}

	staff := newClient(t, a.Handler())
	staff.login("/admin/login", "helper", "unrelated-and-long")
	if rec := staff.get("/admin/widgets"); rec.Code != http.StatusOK {
		t.Errorf("the assigned role should grant access: %d", rec.Code)
	}
}

func TestRolesSectionIsHiddenWhenPermissionsAreOff(t *testing.T) {
	a := permissionApp(t, func(s *settings.Settings) {
		s.Admin.SiteName = "Test admin"
		s.Auth.Permissions = false
	})
	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	admin.Mount(a)

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")

	if body := c.get("/admin/").Body.String(); strings.Contains(body, "/admin/roles") {
		t.Error("the sidebar should not offer roles when permissions are off")
	}
	if rec := c.get("/admin/roles"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when permissions are off", rec.Code)
	}
}
