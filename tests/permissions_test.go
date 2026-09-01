package tests

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

type Widget struct {
	ID    string `gorm:"primaryKey;size:64"`
	Name  string `gorm:"size:100;not null"`
	Price float64
}

type widgetResource struct{}

func (widgetResource) Entity() any { return Widget{} }

func permissionApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	a := app.NewFrom(devSettings(t, fns...))
	a.RegisterModel(model.Of(Widget{}))
	syncSchema(t, a)
	a.Get("/test-signin/{id}", func(w http.ResponseWriter, r *http.Request) {
		target, err := a.Auth.Users().ByID(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, "no such user", http.StatusNotFound)
			return
		}
		if err := a.Auth.Login(r, target); err != nil {
			http.Error(w, "login failed", http.StatusInternalServerError)
		}
	})
	return a
}

func grant(t *testing.T, a *app.App, userID string, codenames ...string) {
	t.Helper()
	store := a.Auth.Permissions()
	if store == nil {
		t.Fatal("no permission store")
	}

	role := &auth.Role{Name: "role-" + userID + "-" + strings.Join(codenames, "-")}
	if err := store.CreateRole(t.Context(), role); err != nil {
		t.Fatal(err)
	}

	all, err := store.AllPermissions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, permission := range all {
		if slices.Contains(codenames, permission.Codename()) {
			ids = append(ids, permission.ID)
		}
	}
	if len(ids) != len(codenames) {
		t.Fatalf("wanted %v, found %d matching permissions", codenames, len(ids))
	}
	if err := store.SetRolePermissions(t.Context(), role.ID, ids); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserRoles(t.Context(), userID, []string{role.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionModelsAreRegisteredOnlyWhenEnabled(t *testing.T) {
	on := permissionApp(t)
	tables := map[string]bool{}
	for _, m := range on.Models() {
		schema, err := on.Describe(m.Entity)
		if err != nil {
			t.Fatal(err)
		}
		tables[schema.Table] = true
	}
	for _, want := range []string{"permissions", "roles", "role_permissions", "user_roles"} {
		if !tables[want] {
			t.Errorf("%s should be registered", want)
		}
	}

	off := permissionApp(t, func(s *settings.Settings) { s.Auth.Permissions = false })
	for _, m := range off.Models() {
		if _, ok := m.Entity.(auth.Permission); ok {
			t.Error("permissions should not be registered when disabled")
		}
	}
	if off.Auth.Permissions() != nil {
		t.Error("a disabled permission store should be nil")
	}
}

func TestSyncCreatesFourPermissionsPerModel(t *testing.T) {
	a := permissionApp(t)

	report, err := a.SyncPermissions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"widgets.create", "widgets.read", "widgets.update", "widgets.delete"} {
		if !slices.Contains(report.Created, want) {
			t.Errorf("missing %s in %v", want, report.Created)
		}
	}

	again, err := a.SyncPermissions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Created) != 0 {
		t.Errorf("a second sync should create nothing, got %v", again.Created)
	}
}

func TestSyncSkipsInternalJoinTables(t *testing.T) {
	a := permissionApp(t, func(s *settings.Settings) { s.Sessions.Backend = settings.SessionsInDB })

	resources, err := a.PermissionResources()
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"sessions", "role_permissions", "user_roles"} {
		if slices.Contains(resources, unwanted) {
			t.Errorf("%s should not get its own permissions", unwanted)
		}
	}
	if !slices.Contains(resources, "users") {
		t.Error("users should get permissions")
	}
}

func TestSyncReportsStalePermissionsWithoutDeletingThem(t *testing.T) {
	a := permissionApp(t)
	if _, err := a.SyncPermissions(t.Context()); err != nil {
		t.Fatal(err)
	}

	store := a.Auth.Permissions()
	before, err := store.AllPermissions(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	report, err := store.SyncPermissions(t.Context(), []string{"users"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(report.Stale, "widgets.read") {
		t.Errorf("a model that vanished should be reported, got %v", report.Stale)
	}

	after, err := store.AllPermissions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("stale permissions must not be deleted: %d before, %d after", len(before), len(after))
	}
}

func TestCanFollowsRolesAndSuperadmin(t *testing.T) {
	a := permissionApp(t)
	if _, err := a.SyncPermissions(t.Context()); err != nil {
		t.Fatal(err)
	}

	root, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true})
	if err != nil {
		t.Fatal(err)
	}
	grant(t, a, helper.ID, "widgets.read")

	seen := map[string]bool{}
	a.Get("/check", func(w http.ResponseWriter, r *http.Request) {
		seen["read"] = a.Auth.Can(r, "widgets.read")
		seen["delete"] = a.Auth.Can(r, "widgets.delete")
	})

	for _, testcase := range []struct {
		name              string
		user              *auth.User
		read, deleteAllow bool
	}{
		{"superadmin", root, true, true},
		{"staff with read", helper, true, false},
	} {
		clear(seen)
		c := newClient(t, a.Handler())
		signInAs(t, c, testcase.user)
		c.get("/check")

		if seen["read"] != testcase.read || seen["delete"] != testcase.deleteAllow {
			t.Errorf("%s: read=%t delete=%t, want %t/%t",
				testcase.name, seen["read"], seen["delete"], testcase.read, testcase.deleteAllow)
		}
	}

	clear(seen)
	newClient(t, a.Handler()).get("/check")
	if seen["read"] {
		t.Error("an anonymous request must not hold permissions")
	}
}

func TestRequirePermissionMiddleware(t *testing.T) {
	a := permissionApp(t)
	if _, err := a.SyncPermissions(t.Context()); err != nil {
		t.Fatal(err)
	}
	helper, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true})
	if err != nil {
		t.Fatal(err)
	}
	grant(t, a, helper.ID, "widgets.read")

	a.Get("/widgets", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) },
		a.Auth.RequirePermission("widgets.read"))
	a.Get("/widgets/delete", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) },
		a.Auth.RequirePermission("widgets.delete"))

	anonymous := newClient(t, a.Handler())
	if rec := anonymous.get("/widgets"); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", rec.Code)
	}

	c := newClient(t, a.Handler())
	signInAs(t, c, helper)
	if rec := c.get("/widgets"); rec.Code != http.StatusOK {
		t.Errorf("granted: %d, want 200", rec.Code)
	}
	if rec := c.get("/widgets/delete"); rec.Code != http.StatusForbidden {
		t.Errorf("not granted: %d, want 403", rec.Code)
	}
}

func TestAdminResourcesFollowPermissions(t *testing.T) {
	a := permissionApp(t, func(s *settings.Settings) { s.Admin.SiteName = "Test admin" })
	if _, err := a.SyncPermissions(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	helper, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true})
	if err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(widgetResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "helper", "unrelated-and-long")

	if rec := c.get("/admin/widgets"); rec.Code != http.StatusForbidden {
		t.Errorf("staff without permission: %d, want 403", rec.Code)
	}
	if body := c.get("/admin/").Body.String(); strings.Contains(body, "/admin/widgets") {
		t.Error("the sidebar should hide a resource the user cannot read")
	}

	grant(t, a, helper.ID, "widgets.read")

	if rec := c.get("/admin/widgets"); rec.Code != http.StatusOK {
		t.Errorf("staff with read: %d, want 200", rec.Code)
	}
	if body := c.get("/admin/").Body.String(); !strings.Contains(body, "/admin/widgets") {
		t.Error("the sidebar should show a resource the user can read")
	}
	if rec := c.get("/admin/widgets/new"); rec.Code != http.StatusForbidden {
		t.Errorf("create without permission: %d, want 403", rec.Code)
	}

	root := newClient(t, a.Handler())
	root.login("/admin/login", "root", "unrelated-and-long")
	for _, target := range []string{"/admin/widgets", "/admin/widgets/new"} {
		if rec := root.get(target); rec.Code != http.StatusOK {
			t.Errorf("superadmin %s: %d, want 200", target, rec.Code)
		}
	}
}

func TestDisabledPermissionsLeaveTheAdminOpenToStaff(t *testing.T) {
	a := permissionApp(t, func(s *settings.Settings) {
		s.Admin.SiteName = "Test admin"
		s.Auth.Permissions = false
	})
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true}); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(widgetResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "helper", "unrelated-and-long")

	if rec := c.get("/admin/widgets"); rec.Code != http.StatusOK {
		t.Errorf("with permissions off, staff should reach everything: %d", rec.Code)
	}
}

func signInAs(t *testing.T, c *client, user *auth.User) {
	t.Helper()
	if rec := c.get("/test-signin/" + user.ID); rec.Code != http.StatusOK {
		t.Fatalf("sign-in helper returned %d", rec.Code)
	}
}
