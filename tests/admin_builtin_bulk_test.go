package tests

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func builtinPortal(t *testing.T) (*app.App, *client) {
	t.Helper()
	a := newTestApp(t, func(s *settings.Settings) { s.Admin.SiteName = "Test admin" })

	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two", "three"} {
		if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: name, Password: "unrelated-and-long"}); err != nil {
			t.Fatal(err)
		}
	}
	admin.Mount(a)

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return a, c
}

func idOf(t *testing.T, a *app.App, username string) string {
	t.Helper()
	user, err := a.Auth.Users().ByUsername(t.Context(), username)
	if err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func TestEveryAdminListOffersBulkSelection(t *testing.T) {
	a, c := builtinPortal(t)
	if _, err := a.SyncPermissions(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := a.Auth.Permissions().CreateRole(t.Context(), &auth.Role{Name: "Viewer"}); err != nil {
		t.Fatal(err)
	}

	second := newClient(t, a.Handler())
	second.login("/admin/login", "root", "unrelated-and-long")

	for _, page := range []string{"/admin/users", "/admin/roles", "/admin/sessions"} {
		body := c.get(page).Body.String()
		if !strings.Contains(body, `name="ids"`) {
			t.Errorf("%s has no selection checkboxes", page)
		}
		if !strings.Contains(body, "With selected") {
			t.Errorf("%s has no bulk bar", page)
		}
	}
}

func TestBulkDeletingUsers(t *testing.T) {
	a, c := builtinPortal(t)

	rec := c.do(http.MethodPost, "/admin/users/bulk", url.Values{
		"csrf_token": {c.token("/admin/users")},
		"action":     {"delete"},
		"ids":        {idOf(t, a, "one"), idOf(t, a, "two")},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("bulk delete: %d, body: %s", rec.Code, rec.Body.String())
	}

	if _, err := a.Auth.Users().ByUsername(t.Context(), "one"); err == nil {
		t.Error("the selected users should be gone")
	}
	if _, err := a.Auth.Users().ByUsername(t.Context(), "three"); err != nil {
		t.Error("unselected users should remain")
	}
}

func TestBulkDeleteSkipsYourOwnAccount(t *testing.T) {
	a, c := builtinPortal(t)
	root := idOf(t, a, "root")

	rec := c.do(http.MethodPost, "/admin/users/bulk", url.Values{
		"csrf_token": {c.token("/admin/users")},
		"action":     {"delete"},
		"ids":        {root, idOf(t, a, "one")},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}

	if _, err := a.Auth.Users().ByID(t.Context(), root); err != nil {
		t.Error("you must not be able to delete yourself in bulk")
	}
	if _, err := a.Auth.Users().ByUsername(t.Context(), "one"); err == nil {
		t.Error("the other selection should still have gone")
	}
	if body := c.get("/admin/users").Body.String(); !strings.Contains(body, "own account was left alone") {
		t.Error("the operator should be told their account was skipped")
	}
}

func TestTheUserListHidesYourOwnCheckbox(t *testing.T) {
	a, c := builtinPortal(t)
	root := idOf(t, a, "root")

	body := c.get("/admin/users").Body.String()
	if strings.Contains(body, `value="`+root+`"`) {
		t.Error("your own row should not be selectable")
	}
	if !strings.Contains(body, `value="`+idOf(t, a, "one")+`"`) {
		t.Error("other rows should be selectable")
	}
}

func TestBulkRevokingSessions(t *testing.T) {
	a, c := builtinPortal(t)

	other := newClient(t, a.Handler())
	other.login("/admin/login", "root", "unrelated-and-long")

	sessions, ok := a.ManageableSessions()
	if !ok {
		t.Skip("session store is not manageable")
	}
	before := sessionCount(t, sessions)
	if before < 2 {
		t.Fatalf("expected at least two sessions, got %d", before)
	}

	ids := []string{}
	for _, s := range allSessions(t, sessions) {
		ids = append(ids, s.ID())
	}

	rec := c.do(http.MethodPost, "/admin/sessions/bulk", url.Values{
		"csrf_token": {c.token("/admin/sessions")},
		"ids":        ids,
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if sessionCount(t, sessions) >= before {
		t.Errorf("sessions should have been revoked: %d before, %d after", before, sessionCount(t, sessions))
	}
}

func TestBulkWithNoSelectionOnBuiltInLists(t *testing.T) {
	a, c := builtinPortal(t)

	for _, page := range []string{"/admin/users/bulk", "/admin/roles/bulk", "/admin/sessions/bulk"} {
		rec := c.do(http.MethodPost, page, url.Values{
			"csrf_token": {c.token("/admin/users")},
			"action":     {"delete"},
		})
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s = %d, want a redirect", page, rec.Code)
		}
	}

	if _, err := a.Auth.Users().ByUsername(t.Context(), "one"); err != nil {
		t.Error("nothing should have been deleted")
	}
}

func TestBulkRefusesAnUnknownActionOnUsers(t *testing.T) {
	a, c := builtinPortal(t)

	rec := c.do(http.MethodPost, "/admin/users/bulk", url.Values{
		"csrf_token": {c.token("/admin/users")},
		"action":     {"disable-everything"},
		"ids":        {idOf(t, a, "one")},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if _, err := a.Auth.Users().ByUsername(t.Context(), "one"); err != nil {
		t.Error("an unknown action must not delete anything")
	}
}

func TestStaffCannotBulkDeleteUsers(t *testing.T) {
	a, _ := builtinPortal(t)
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true}); err != nil {
		t.Fatal(err)
	}

	staff := newClient(t, a.Handler())
	staff.login("/admin/login", "helper", "unrelated-and-long")

	rec := staff.do(http.MethodPost, "/admin/users/bulk", url.Values{
		"action": {"delete"},
		"ids":    {idOf(t, a, "one")},
	})
	if rec.Code == http.StatusSeeOther {
		t.Error("staff must not reach user bulk delete")
	}
	if _, err := a.Auth.Users().ByUsername(t.Context(), "one"); err != nil {
		t.Error("nothing should have been deleted")
	}
}
