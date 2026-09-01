package tests

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func setupStaffAdmin(t *testing.T, fns ...func(*settings.Settings)) *client {
	t.Helper()
	base := func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }
	a := newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)

	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "supersecret"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "helper", Password: "supersecret", IsStaff: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "plain", Password: "supersecret"}); err != nil {
		t.Fatal(err)
	}
	admin.Mount(a)
	return newClient(t, a.Handler())
}

func TestStaffCanReachTheAdminPortal(t *testing.T) {
	c := setupStaffAdmin(t)

	if res := c.login("/admin/login", "helper", "supersecret"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("staff login: %d, want 303", res.StatusCode)
	}
	if rec := c.get("/admin/"); rec.Code != http.StatusOK {
		t.Errorf("staff dashboard: %d, want 200", rec.Code)
	}
}

func TestPlainUserStillCannotReachTheAdminPortal(t *testing.T) {
	c := setupStaffAdmin(t)

	if res := c.login("/admin/login", "plain", "supersecret"); res.StatusCode != http.StatusForbidden {
		t.Errorf("plain login: %d, want 403", res.StatusCode)
	}
}

func TestStaffCannotManageUsersOrSessions(t *testing.T) {
	c := setupStaffAdmin(t)
	c.login("/admin/login", "helper", "supersecret")

	for _, target := range []string{"/admin/users", "/admin/users/new", "/admin/sessions"} {
		if rec := c.get(target); rec.Code != http.StatusForbidden {
			t.Errorf("staff GET %s: %d, want 403", target, rec.Code)
		}
	}
}

func TestSuperadminKeepsUserManagement(t *testing.T) {
	c := setupStaffAdmin(t)
	c.login("/admin/login", "root", "supersecret")

	for _, target := range []string{"/admin/users", "/admin/sessions"} {
		if rec := c.get(target); rec.Code != http.StatusOK {
			t.Errorf("superadmin GET %s: %d, want 200", target, rec.Code)
		}
	}
}

func TestSidebarHidesPrivilegedLinksFromStaff(t *testing.T) {
	c := setupStaffAdmin(t)
	c.login("/admin/login", "helper", "supersecret")

	body := c.get("/admin/").Body.String()
	if strings.Contains(body, `href="/admin/users"`) {
		t.Error("staff should not be shown a link they cannot follow")
	}
	if !strings.Contains(body, "staff") {
		t.Error("the sidebar should label the signed-in user as staff")
	}
}

func TestSuperadminIsAlwaysStaff(t *testing.T) {
	service, _ := newTestAuth()

	root, err := service.CreateSuperadmin(t.Context(), "root", "root@example.com", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsStaff {
		t.Error("a superadmin must count as staff, or they could not reach the portal")
	}
	if !root.CanReachAdmin() {
		t.Error("CanReachAdmin should be true for a superadmin")
	}
}

func TestCanReachAdminFollowsActiveAndRole(t *testing.T) {
	for _, testcase := range []struct {
		name string
		user *auth.User
		want bool
	}{
		{"nil", nil, false},
		{"plain", &auth.User{IsActive: true}, false},
		{"staff", &auth.User{IsActive: true, IsStaff: true}, true},
		{"superadmin", &auth.User{IsActive: true, IsSuperadmin: true}, true},
		{"disabled staff", &auth.User{IsStaff: true}, false},
		{"disabled superadmin", &auth.User{IsSuperadmin: true}, false},
	} {
		if got := testcase.user.CanReachAdmin(); got != testcase.want {
			t.Errorf("%s: CanReachAdmin = %t, want %t", testcase.name, got, testcase.want)
		}
	}
}

func TestStaffFlagSurvivesTheAdminUserForm(t *testing.T) {
	c := setupStaffAdmin(t)
	c.login("/admin/login", "root", "supersecret")

	token := c.token("/admin/users/new")
	rec := c.do(http.MethodPost, "/admin/users/new", url.Values{
		"csrf_token": {token},
		"username":   {"editor"},
		"password":   {"supersecret"},
		"is_active":  {"1"},
		"is_staff":   {"1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating a staff user: %d, want 303", rec.Code)
	}

	if body := c.get("/admin/users").Body.String(); !strings.Contains(body, "staff") {
		t.Error("the user list should mark the new account as staff")
	}
}

func TestLastSuperadminCannotBeStrippedOfStaff(t *testing.T) {
	service, _ := newTestAuth()
	store := auth.Guarded(service.Users())

	root, err := service.CreateSuperadmin(t.Context(), "root", "root@example.com", "supersecret")
	if err != nil {
		t.Fatal(err)
	}

	demoted := root.Clone()
	demoted.IsStaff = false
	if err := store.Update(t.Context(), demoted); err == nil {
		t.Error("the last active superadmin must not lose portal access")
	}
}
