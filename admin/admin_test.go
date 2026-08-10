package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote"
	"github.com/farhapartex/coyote/settings"
)

var tokenPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

type client struct {
	t       *testing.T
	handler http.Handler
	cookie  *http.Cookie
}

func (c *client) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	c.t.Helper()
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	if cookies := rec.Result().Cookies(); len(cookies) > 0 {
		c.cookie = cookies[0]
	}
	return rec
}

func (c *client) token(target string) string {
	c.t.Helper()
	body := c.do(http.MethodGet, target, nil).Body.String()
	match := tokenPattern.FindStringSubmatch(body)
	if match == nil {
		c.t.Fatalf("no csrf token on %s", target)
	}
	return match[1]
}

func newApp(t *testing.T, fns ...func(*settings.Settings)) *coyote.App {
	t.Helper()
	base := func(s *settings.Settings) {
		s.Debug = true
		s.SecretKey = "test-secret-key-that-is-long-enough-to-pass"
		s.AllowedHosts = []string{"*"}
		s.Auth.PBKDF2Iterations = 1000
		s.Admin.SiteName = "Test admin"
		s.Templates.FS = fstest.MapFS{
			"layouts/base.html": &fstest.MapFile{Data: []byte(`{{block "content" .}}{{end}}`)},
		}
	}
	resolved, err := settings.New(append([]func(*settings.Settings){base}, fns...)...)
	if err != nil {
		t.Fatalf("building settings: %v", err)
	}
	return coyote.NewFrom(resolved)
}

func setup(t *testing.T, fns ...func(*settings.Settings)) (*coyote.App, *client) {
	t.Helper()
	app := newApp(t, fns...)
	if _, err := app.Auth.CreateUser("root", "root@example.com", "supersecret", true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Auth.CreateUser("plain", "", "supersecret", false, false); err != nil {
		t.Fatal(err)
	}
	Mount(app)
	return app, &client{t: t, handler: app.Handler()}
}

func (c *client) login(username, password string) *httptest.ResponseRecorder {
	c.t.Helper()
	token := c.token("/admin/login")
	return c.do(http.MethodPost, "/admin/login", url.Values{
		"csrf_token": {token},
		"username":   {username},
		"password":   {password},
	})
}

func TestAdminRequiresStaff(t *testing.T) {
	_, c := setup(t)

	rec := c.do(http.MethodGet, "/admin/", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("anonymous dashboard: %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/admin/login?next=%2Fadmin%2F" {
		t.Errorf("redirect = %q", got)
	}

	rec = c.login("plain", "supersecret")
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-staff login: %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "does not have access") {
		t.Error("expected access message for non-staff")
	}
}

func TestAdminLoginRejectsBadPassword(t *testing.T) {
	_, c := setup(t)
	rec := c.login("root", "wrongpassword")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid username or password") {
		t.Error("expected invalid credentials message")
	}
}

func TestAdminLoginWithoutCSRFIsRejected(t *testing.T) {
	_, c := setup(t)
	c.do(http.MethodGet, "/admin/login", nil)
	rec := c.do(http.MethodPost, "/admin/login", url.Values{
		"username": {"root"},
		"password": {"supersecret"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("code %d, want 403", rec.Code)
	}
}

func TestAdminUserLifecycle(t *testing.T) {
	app, c := setup(t)

	if rec := c.login("root", "supersecret"); rec.Code != http.StatusSeeOther {
		t.Fatalf("login: %d, want 303", rec.Code)
	}
	if rec := c.do(http.MethodGet, "/admin/", nil); rec.Code != http.StatusOK {
		t.Fatalf("dashboard: %d, want 200", rec.Code)
	}

	token := c.token("/admin/users/new")
	rec := c.do(http.MethodPost, "/admin/users/new", url.Values{
		"csrf_token": {token},
		"username":   {"jane"},
		"full_name":  {"Jane Doe"},
		"email":      {"jane@example.com"},
		"password":   {"supersecret"},
		"is_active":  {"1"},
		"is_staff":   {"1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, want 303", rec.Code)
	}

	jane, err := app.Auth.Users().ByUsername("jane")
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if !jane.IsStaff || !jane.IsActive || jane.FullName != "Jane Doe" {
		t.Errorf("unexpected user: %+v", jane)
	}

	token = c.token("/admin/users/" + jane.ID)
	rec = c.do(http.MethodPost, "/admin/users/"+jane.ID, url.Values{
		"csrf_token": {token},
		"username":   {"jane"},
		"full_name":  {"Jane Q Doe"},
		"email":      {"jane@corp.com"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d, want 303", rec.Code)
	}
	jane, _ = app.Auth.Users().ByUsername("jane")
	if jane.FullName != "Jane Q Doe" || jane.Email != "jane@corp.com" {
		t.Errorf("update did not apply: %+v", jane)
	}
	if jane.IsStaff {
		t.Error("unchecked is_staff should clear the flag")
	}

	token = c.token("/admin/users")
	rec = c.do(http.MethodPost, "/admin/users/"+jane.ID+"/delete", url.Values{"csrf_token": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d, want 303", rec.Code)
	}
	if _, err := app.Auth.Users().ByUsername("jane"); err == nil {
		t.Error("user should be deleted")
	}
}

func TestAdminCannotDeleteSelf(t *testing.T) {
	app, c := setup(t)
	c.login("root", "supersecret")
	root, _ := app.Auth.Users().ByUsername("root")

	token := c.token("/admin/users")
	c.do(http.MethodPost, "/admin/users/"+root.ID+"/delete", url.Values{"csrf_token": {token}})

	if _, err := app.Auth.Users().ByUsername("root"); err != nil {
		t.Error("self-deletion should be refused")
	}
}

func TestAdminLogoutEndsSession(t *testing.T) {
	_, c := setup(t)
	c.login("root", "supersecret")

	token := c.token("/admin/users")
	if rec := c.do(http.MethodPost, "/admin/logout", url.Values{"csrf_token": {token}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d, want 303", rec.Code)
	}
	if rec := c.do(http.MethodGet, "/admin/", nil); rec.Code != http.StatusSeeOther {
		t.Errorf("after logout: %d, want 303 to login", rec.Code)
	}
}

func TestAdminSettingsPageRedactsSecretKey(t *testing.T) {
	secret := "super-secret-key-that-must-never-be-shown"
	_, c := setup(t, func(s *settings.Settings) {
		s.SecretKey = secret
		s.Sessions.CookieName = "shown_cookie"
	})
	c.login("root", "supersecret")

	rec := c.do(http.MethodGet, "/admin/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, secret) {
		t.Error("SecretKey must never be rendered")
	}
	if !strings.Contains(body, "characters hidden") {
		t.Error("expected the redacted SecretKey placeholder")
	}
	if !strings.Contains(body, "shown_cookie") {
		t.Error("expected non-secret settings to be shown")
	}
}

func TestAdminPrefixComesFromSettings(t *testing.T) {
	app := newApp(t, func(s *settings.Settings) { s.Admin.Prefix = "/control" })
	if _, err := app.Auth.CreateUser("root", "", "supersecret", true, true); err != nil {
		t.Fatal(err)
	}
	portal := Mount(app)
	if portal.Prefix() != "/control" {
		t.Errorf("prefix = %q", portal.Prefix())
	}
	c := &client{t: t, handler: app.Handler()}
	if rec := c.do(http.MethodGet, "/control/login", nil); rec.Code != http.StatusOK {
		t.Errorf("login page at settings prefix: %d", rec.Code)
	}
	if rec := c.do(http.MethodGet, "/admin/login", nil); rec.Code != http.StatusNotFound {
		t.Errorf("default prefix should not exist: %d", rec.Code)
	}
}

func TestAdminUnknownUserIs404(t *testing.T) {
	_, c := setup(t)
	c.login("root", "supersecret")
	if rec := c.do(http.MethodGet, "/admin/users/does-not-exist", nil); rec.Code != http.StatusNotFound {
		t.Errorf("code %d, want 404", rec.Code)
	}
}

func TestRegisteredSectionIsGuardedAndMounted(t *testing.T) {
	app := newApp(t, func(s *settings.Settings) {
		s.Admin.Prefix = "/backoffice"
		s.Admin.SiteName = "Backoffice"
	})
	if _, err := app.Auth.CreateUser("root", "", "supersecret", true, true); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, handler: app.Handler()}

	portal := Mount(app)
	portal.Register(Section{
		Name: "Reports",
		Slug: "reports",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("report body"))
		}),
	})
	c.handler = app.Handler()

	if rec := c.do(http.MethodGet, "/backoffice/s/reports/", nil); rec.Code != http.StatusSeeOther {
		t.Errorf("anonymous section: %d, want 303", rec.Code)
	}

	token := c.token("/backoffice/login")
	c.do(http.MethodPost, "/backoffice/login", url.Values{
		"csrf_token": {token},
		"username":   {"root"},
		"password":   {"supersecret"},
	})

	rec := c.do(http.MethodGet, "/backoffice/s/reports/", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "report body" {
		t.Errorf("section: %d %q", rec.Code, rec.Body.String())
	}
}
