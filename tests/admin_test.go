package tests

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func setupAdmin(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *client) {
	t.Helper()
	base := func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }
	a := newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "supersecret"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "plain", Password: "supersecret"}); err != nil {
		t.Fatal(err)
	}
	admin.Mount(a)
	return a, newClient(t, a.Handler())
}

func (c *client) login(loginPath, username, password string) *http.Response {
	c.t.Helper()
	token := c.token(loginPath)
	rec := c.do(http.MethodPost, loginPath, url.Values{
		"csrf_token": {token},
		"username":   {username},
		"password":   {password},
	})
	return rec.Result()
}

func TestAdminRequiresSuperadmin(t *testing.T) {
	_, c := setupAdmin(t)

	rec := c.get("/admin/")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("anonymous dashboard: %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/admin/login?next=%2Fadmin%2F" {
		t.Errorf("redirect = %q", got)
	}

	res := c.login("/admin/login", "plain", "supersecret")
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("non-superadmin login: %d, want 403", res.StatusCode)
	}
}

func TestAdminLoginRejectsBadPassword(t *testing.T) {
	_, c := setupAdmin(t)
	token := c.token("/admin/login")
	rec := c.do(http.MethodPost, "/admin/login", url.Values{
		"csrf_token": {token},
		"username":   {"root"},
		"password":   {"wrongpassword"},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid username or password") {
		t.Error("expected invalid credentials message")
	}
}

func TestAdminLoginWithoutCSRFIsRejected(t *testing.T) {
	_, c := setupAdmin(t)
	c.get("/admin/login")
	rec := c.do(http.MethodPost, "/admin/login", url.Values{
		"username": {"root"},
		"password": {"supersecret"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("code %d, want 403", rec.Code)
	}
}

func TestAdminUserLifecycle(t *testing.T) {
	a, c := setupAdmin(t)

	if res := c.login("/admin/login", "root", "supersecret"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %d, want 303", res.StatusCode)
	}
	if rec := c.get("/admin/"); rec.Code != http.StatusOK {
		t.Fatalf("dashboard: %d, want 200", rec.Code)
	}

	token := c.token("/admin/users/new")
	rec := c.do(http.MethodPost, "/admin/users/new", url.Values{
		"csrf_token":    {token},
		"username":      {"jane"},
		"first_name":    {"Jane"},
		"last_name":     {"Doe"},
		"email":         {"jane@example.com"},
		"password":      {"supersecret"},
		"is_active":     {"1"},
		"is_superadmin": {"1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, want 303", rec.Code)
	}

	jane, err := a.Auth.Users().ByUsername(t.Context(), "jane")
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if !jane.IsSuperadmin || !jane.IsActive {
		t.Errorf("flags not applied: %+v", jane)
	}
	if jane.FirstName != "Jane" || jane.LastName != "Doe" || jane.FullName() != "Jane Doe" {
		t.Errorf("names not applied: %+v", jane)
	}
	if jane.Password == "supersecret" {
		t.Error("the admin form stored a plain text password")
	}

	token = c.token("/admin/users/" + jane.ID)
	rec = c.do(http.MethodPost, "/admin/users/"+jane.ID, url.Values{
		"csrf_token": {token},
		"username":   {"jane"},
		"first_name": {"Jane"},
		"last_name":  {"Q Doe"},
		"email":      {"jane@corp.com"},
		"is_active":  {"1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d, want 303", rec.Code)
	}
	jane, _ = a.Auth.Users().ByUsername(t.Context(), "jane")
	if jane.FullName() != "Jane Q Doe" || jane.Email != "jane@corp.com" {
		t.Errorf("update did not apply: %+v", jane)
	}
	if jane.IsSuperadmin {
		t.Error("unchecked is_superadmin should clear the flag")
	}

	token = c.token("/admin/users")
	rec = c.do(http.MethodPost, "/admin/users/"+jane.ID+"/delete", url.Values{"csrf_token": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d, want 303", rec.Code)
	}
	if _, err := a.Auth.Users().ByUsername(t.Context(), "jane"); err == nil {
		t.Error("user should be deleted")
	}
}

func TestAdminCannotDeleteSelf(t *testing.T) {
	a, c := setupAdmin(t)
	c.login("/admin/login", "root", "supersecret")
	root, _ := a.Auth.Users().ByUsername(t.Context(), "root")

	token := c.token("/admin/users")
	c.do(http.MethodPost, "/admin/users/"+root.ID+"/delete", url.Values{"csrf_token": {token}})

	if _, err := a.Auth.Users().ByUsername(t.Context(), "root"); err != nil {
		t.Error("self-deletion should be refused")
	}
}

func TestAdminLogoutEndsSession(t *testing.T) {
	_, c := setupAdmin(t)
	c.login("/admin/login", "root", "supersecret")

	token := c.token("/admin/users")
	if rec := c.do(http.MethodPost, "/admin/logout", url.Values{"csrf_token": {token}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d, want 303", rec.Code)
	}
	if rec := c.get("/admin/"); rec.Code != http.StatusSeeOther {
		t.Errorf("after logout: %d, want 303 to login", rec.Code)
	}
}

func TestAdminPrefixComesFromSettings(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) { s.Admin.Prefix = "/control" })
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "", "supersecret"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	if portal.Prefix() != "/control" {
		t.Errorf("prefix = %q", portal.Prefix())
	}
	c := newClient(t, a.Handler())
	if rec := c.get("/control/login"); rec.Code != http.StatusOK {
		t.Errorf("login page at settings prefix: %d", rec.Code)
	}
	if rec := c.get("/admin/login"); rec.Code != http.StatusNotFound {
		t.Errorf("default prefix should not exist: %d", rec.Code)
	}
}

func TestAdminUnknownUserIs404(t *testing.T) {
	_, c := setupAdmin(t)
	c.login("/admin/login", "root", "supersecret")
	if rec := c.get("/admin/users/does-not-exist"); rec.Code != http.StatusNotFound {
		t.Errorf("code %d, want 404", rec.Code)
	}
}

func TestRegisteredSectionIsGuardedAndMounted(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Admin.Prefix = "/backoffice"
		s.Admin.SiteName = "Backoffice"
	})
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "", "supersecret"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.Register(admin.Section{
		Name: "Reports",
		Slug: "reports",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("report body"))
		}),
	})
	c := newClient(t, a.Handler())

	if rec := c.get("/backoffice/s/reports/"); rec.Code != http.StatusSeeOther {
		t.Errorf("anonymous section: %d, want 303", rec.Code)
	}

	c.login("/backoffice/login", "root", "supersecret")

	rec := c.get("/backoffice/s/reports/")
	if rec.Code != http.StatusOK || rec.Body.String() != "report body" {
		t.Errorf("section: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRemovedDeveloperPagesAreGone(t *testing.T) {
	_, c := setupAdmin(t)
	c.login("/admin/login", "root", "supersecret")

	for _, path := range []string{"/admin/settings", "/admin/routes"} {
		if rec := c.get(path); rec.Code != http.StatusNotFound {
			t.Errorf("%s returned %d, want 404", path, rec.Code)
		}
	}

	body := c.get("/admin/").Body.String()
	for _, href := range []string{"/admin/routes", "/admin/settings"} {
		if strings.Contains(body, href) {
			t.Errorf("the admin nav still links to %s", href)
		}
	}

	if rec := c.get("/admin/nonsense"); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown admin path returned %d, want 404", rec.Code)
	}
	if rec := c.get("/admin/"); rec.Code != http.StatusOK {
		t.Errorf("the dashboard returned %d, want 200", rec.Code)
	}
}

func TestTheUserListPaginatesInsteadOfLoadingEveryRow(t *testing.T) {
	a, c := setupAdmin(t, func(s *settings.Settings) { s.Pagination.PerPage = 5 })
	c.login("/admin/login", "root", "supersecret")

	for i := range 12 {
		if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{
			Username: fmt.Sprintf("user%02d", i), Password: "unrelated-and-long",
		}); err != nil {
			t.Fatal(err)
		}
	}

	first := c.get("/admin/users").Body.String()
	if strings.Count(first, `name="ids"`) > 5 {
		t.Errorf("the first page rendered %d rows, want at most 5", strings.Count(first, `name="ids"`))
	}
	if !strings.Contains(first, "page=2") {
		t.Error("the list should offer a second page")
	}

	second := c.get("/admin/users?page=2").Body.String()
	if second == first {
		t.Error("page 2 rendered the same rows as page 1")
	}
}

func TestTheUserSearchFiltersInTheDatabase(t *testing.T) {
	a, c := setupAdmin(t, func(s *settings.Settings) { s.Pagination.PerPage = 50 })
	c.login("/admin/login", "root", "supersecret")

	for _, name := range []string{"ada", "grace", "alan"} {
		if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{
			Username: name, Email: name + "@example.test", Password: "unrelated-and-long",
		}); err != nil {
			t.Fatal(err)
		}
	}

	body := c.get("/admin/users?q=grace").Body.String()
	if !strings.Contains(body, "grace") {
		t.Error("the match should be listed")
	}
	for _, absent := range []string{">ada<", ">alan<"} {
		if strings.Contains(body, absent) {
			t.Errorf("%s should not survive the search", absent)
		}
	}

	empty := c.get("/admin/users?q=nobody-by-that-name").Body.String()
	if !strings.Contains(empty, "No users match") {
		t.Error("a search with no matches should say so")
	}
}

func TestASearchTermIsNotTreatedAsAPattern(t *testing.T) {
	a, c := setupAdmin(t, func(s *settings.Settings) { s.Pagination.PerPage = 50 })
	c.login("/admin/login", "root", "supersecret")

	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{
		Username: "ada", Password: "unrelated-and-long",
	}); err != nil {
		t.Fatal(err)
	}

	for _, term := range []string{"%", "_", "%%", "a%a"} {
		body := c.get("/admin/users?q=" + url.QueryEscape(term)).Body.String()
		if strings.Contains(body, ">ada<") {
			t.Errorf("the term %q matched ada as a wildcard", term)
		}
	}
}

func TestTheDashboardCountsWithoutReadingEveryUser(t *testing.T) {
	a, c := setupAdmin(t)
	c.login("/admin/login", "root", "supersecret")

	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{
		Username: "helper", Password: "unrelated-and-long", IsStaff: true,
	}); err != nil {
		t.Fatal(err)
	}

	stats, err := a.Auth.Users().Stats(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 3 || stats.Superadmins != 1 || stats.Staff != 1 {
		t.Errorf("stats = %+v, want 3 total, 1 superadmin, 1 staff", stats)
	}

	recent, err := a.Auth.Users().Recent(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("Recent returned %d users, want 2", len(recent))
	}
	if recent[0].CreatedAt.Before(recent[1].CreatedAt) {
		t.Error("Recent should be newest first")
	}

	if rec := c.get("/admin/"); rec.Code != http.StatusOK {
		t.Errorf("the dashboard answered %d", rec.Code)
	}
}
