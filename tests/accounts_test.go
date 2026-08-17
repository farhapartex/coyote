package tests

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/accounts"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func accountsApp(t *testing.T, opts accounts.Options, fns ...func(*settings.Settings)) (*app.App, *client) {
	t.Helper()
	a := newTestApp(t, fns...)
	accounts.Mount(a, opts)
	if _, err := a.Auth.CreateUser(auth.NewUser{
		Username: "jane", Email: "jane@example.com", Password: "unrelated-and-long",
	}); err != nil {
		t.Fatal(err)
	}
	return a, newClient(t, a.Handler())
}

func TestAccountsSignInAndOut(t *testing.T) {
	_, c := accountsApp(t, accounts.Options{AfterLogin: "/"})

	if rec := c.get("/accounts/login"); rec.Code != http.StatusOK {
		t.Fatalf("login page: %d", rec.Code)
	}

	rec := c.do(http.MethodPost, "/accounts/login", url.Values{
		"csrf_token": {c.token("/accounts/login")},
		"username":   {"jane"},
		"password":   {"unrelated-and-long"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sign in: %d, want 303", rec.Code)
	}

	if body := c.get("/accounts/profile").Body.String(); !strings.Contains(body, "jane") {
		t.Error("the profile page should name the signed-in user")
	}

	if rec := c.do(http.MethodPost, "/accounts/logout", url.Values{
		"csrf_token": {c.token("/accounts/profile")},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("sign out: %d", rec.Code)
	}

	if rec := c.get("/accounts/profile"); rec.Code != http.StatusSeeOther {
		t.Error("the profile page should require a session after signing out")
	}
}

func TestAccountsRejectBadCredentials(t *testing.T) {
	_, c := accountsApp(t, accounts.Options{})

	rec := c.do(http.MethodPost, "/accounts/login", url.Values{
		"csrf_token": {c.token("/accounts/login")},
		"username":   {"jane"},
		"password":   {"wrong"},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid username or password") {
		t.Error("the form should say the credentials were wrong")
	}
}

func TestRegistrationIsOffUnlessAskedFor(t *testing.T) {
	_, c := accountsApp(t, accounts.Options{})

	if rec := c.get("/accounts/register"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when registration is off", rec.Code)
	}
	if body := c.get("/accounts/login").Body.String(); strings.Contains(body, "Create one") {
		t.Error("the login page should not invite signups when they are off")
	}
}

func TestRegistrationCreatesAPlainUserAndSignsThemIn(t *testing.T) {
	a, c := accountsApp(t, accounts.Options{AllowRegistration: true})

	rec := c.do(http.MethodPost, "/accounts/register", url.Values{
		"csrf_token": {c.token("/accounts/register")},
		"username":   {"newcomer"},
		"email":      {"new@example.com"},
		"password":   {"unrelated-and-long"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("register: %d, want 303. body: %s", rec.Code, rec.Body.String())
	}

	user, err := a.Auth.Users().ByUsername("newcomer")
	if err != nil {
		t.Fatal(err)
	}
	if user.IsStaff || user.IsSuperadmin {
		t.Error("self-service accounts must not be staff or superadmin")
	}
	if rec := c.get("/accounts/profile"); rec.Code != http.StatusOK {
		t.Error("registration should sign the new user in")
	}
}

func TestRegistrationAppliesPasswordRulesAndUniqueness(t *testing.T) {
	_, c := accountsApp(t, accounts.Options{AllowRegistration: true})

	rec := c.do(http.MethodPost, "/accounts/register", url.Values{
		"csrf_token": {c.token("/accounts/register")},
		"username":   {"someone"},
		"password":   {"password"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a common password should be refused, got %d", rec.Code)
	}

	rec = c.do(http.MethodPost, "/accounts/register", url.Values{
		"csrf_token": {c.token("/accounts/register")},
		"username":   {"jane"},
		"password":   {"unrelated-and-long"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a taken username should be refused, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "already taken") {
		t.Error("the form should explain the clash")
	}
}

func TestProfileEditingAndPasswordChange(t *testing.T) {
	a, c := accountsApp(t, accounts.Options{})
	c.do(http.MethodPost, "/accounts/login", url.Values{
		"csrf_token": {c.token("/accounts/login")},
		"username":   {"jane"},
		"password":   {"unrelated-and-long"},
	})

	if rec := c.do(http.MethodPost, "/accounts/profile", url.Values{
		"csrf_token": {c.token("/accounts/profile")},
		"first_name": {"Jane"},
		"last_name":  {"Doe"},
		"email":      {"jane.doe@example.com"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("saving the profile: %d", rec.Code)
	}

	user, err := a.Auth.Users().ByUsername("jane")
	if err != nil {
		t.Fatal(err)
	}
	if user.FullName() != "Jane Doe" || user.Email != "jane.doe@example.com" {
		t.Errorf("profile not saved: %+v", user)
	}

	if rec := c.do(http.MethodPost, "/accounts/password", url.Values{
		"csrf_token":       {c.token("/accounts/profile")},
		"current_password": {"wrong"},
		"new_password":     {"another-good-secret"},
		"confirm_password": {"another-good-secret"},
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("a wrong current password should be refused, got %d", rec.Code)
	}

	if rec := c.do(http.MethodPost, "/accounts/password", url.Values{
		"csrf_token":       {c.token("/accounts/profile")},
		"current_password": {"unrelated-and-long"},
		"new_password":     {"another-good-secret"},
		"confirm_password": {"another-good-secret"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("changing the password: %d", rec.Code)
	}

	if _, err := a.Auth.Authenticate("jane", "another-good-secret"); err != nil {
		t.Errorf("the new password should work: %v", err)
	}
}

func TestProfileRequiresASession(t *testing.T) {
	_, c := accountsApp(t, accounts.Options{})

	rec := c.get("/accounts/profile")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want a redirect", rec.Code)
	}
	if got := rec.Header().Get("Location"); !strings.HasPrefix(got, "/accounts/login") {
		t.Errorf("redirect = %q, want the login page", got)
	}
}

func TestAccountsPagesCanBeReplaced(t *testing.T) {
	a := newTestApp(t)
	accounts.Mount(a, accounts.Options{Pages: accounts.Pages{Login: "pages/hello.html"}})

	body := newClient(t, a.Handler()).get("/accounts/login").Body.String()
	if !strings.Contains(body, "<h1>") {
		t.Error("a supplied page should be rendered through the app's own templates")
	}
	if strings.Contains(body, "Sign in</h1>") {
		t.Error("the built-in login page should not be used when one is supplied")
	}
}

func TestLoginNeverWritesStaleUserFields(t *testing.T) {
	a, c := accountsApp(t, accounts.Options{})

	stale, err := a.Auth.Users().ByUsername("jane")
	if err != nil {
		t.Fatal(err)
	}

	if err := a.Auth.SetPassword(stale.ID, "a-brand-new-secret"); err != nil {
		t.Fatal(err)
	}

	a.Get("/stale-login", func(w http.ResponseWriter, r *http.Request) {
		if err := a.Auth.Login(r, stale); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	if rec := c.get("/stale-login"); rec.Code != http.StatusOK {
		t.Fatalf("login: %d", rec.Code)
	}

	if _, err := a.Auth.Authenticate("jane", "a-brand-new-secret"); err != nil {
		t.Errorf("signing in with a stale user object must not revert the stored password: %v", err)
	}
	if stale.LastLoginAt.IsZero() {
		t.Error("Login should still stamp the caller's copy")
	}
}
