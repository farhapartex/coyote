package tests

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/contrib/accounts"
	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func changeApp(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *client) {
	t.Helper()
	base := func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }
	a := newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)

	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateUser(auth.NewUser{Username: "jane", Password: "her-first-secret"}); err != nil {
		t.Fatal(err)
	}
	admin.Mount(a)
	accounts.Mount(a, accounts.Options{})
	return a, newClient(t, a.Handler())
}

func signedInAsJane(t *testing.T, c *client) {
	t.Helper()
	rec := c.do(http.MethodPost, "/accounts/login", url.Values{
		"csrf_token": {c.token("/accounts/login")},
		"username":   {"jane"},
		"password":   {"her-first-secret"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sign in: %d", rec.Code)
	}
}

func changePassword(t *testing.T, c *client, current, next, confirm string) int {
	t.Helper()
	return c.do(http.MethodPost, "/accounts/password", url.Values{
		"csrf_token":       {c.token("/accounts/profile")},
		"current_password": {current},
		"new_password":     {next},
		"confirm_password": {confirm},
	}).Code
}

func TestPasswordChangeIsAllowedByDefault(t *testing.T) {
	if !settings.Default().Auth.AllowPasswordChange {
		t.Fatal("AllowPasswordChange must default to true")
	}

	a, c := changeApp(t)
	if !a.Auth.AllowsPasswordChange() {
		t.Fatal("a project that never mentions the setting should allow changes")
	}
	signedInAsJane(t, c)

	if code := changePassword(t, c, "her-first-secret", "her-second-secret", "her-second-secret"); code != http.StatusSeeOther {
		t.Fatalf("change: %d", code)
	}
	if _, err := a.Auth.Authenticate("jane", "her-second-secret"); err != nil {
		t.Errorf("the new password should work: %v", err)
	}
	if _, err := a.Auth.Authenticate("jane", "her-first-secret"); err == nil {
		t.Error("the old password should stop working")
	}
}

func TestConfirmationMustMatch(t *testing.T) {
	a, c := changeApp(t)
	signedInAsJane(t, c)

	rec := c.do(http.MethodPost, "/accounts/password", url.Values{
		"csrf_token":       {c.token("/accounts/profile")},
		"current_password": {"her-first-secret"},
		"new_password":     {"her-second-secret"},
		"confirm_password": {"typo-here-instead"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "do not match") {
		t.Errorf("the form should say the confirmation failed:\n%s", rec.Body.String())
	}
	if _, err := a.Auth.Authenticate("jane", "her-first-secret"); err != nil {
		t.Error("nothing should have changed")
	}
}

func TestTheCurrentPasswordIsRequired(t *testing.T) {
	a, c := changeApp(t)
	signedInAsJane(t, c)

	if code := changePassword(t, c, "not-her-password", "her-second-secret", "her-second-secret"); code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
	if _, err := a.Auth.Authenticate("jane", "her-first-secret"); err != nil {
		t.Error("the password should be untouched")
	}
}

func TestANewPasswordStillRunsTheRules(t *testing.T) {
	_, c := changeApp(t)
	signedInAsJane(t, c)

	if code := changePassword(t, c, "her-first-secret", "password", "password"); code != http.StatusBadRequest {
		t.Errorf("a common password should be refused, got %d", code)
	}
}

func TestChangingAPasswordRevokesOtherSessions(t *testing.T) {
	a, first := changeApp(t)
	signedInAsJane(t, first)

	second := newClient(t, a.Handler())
	signedInAsJane(t, second)
	if rec := second.get("/accounts/profile"); rec.Code != http.StatusOK {
		t.Fatalf("the second session should start out working: %d", rec.Code)
	}

	if code := changePassword(t, first, "her-first-secret", "her-second-secret", "her-second-secret"); code != http.StatusSeeOther {
		t.Fatalf("change: %d", code)
	}

	if rec := first.get("/accounts/profile"); rec.Code != http.StatusOK {
		t.Errorf("the session that changed the password should survive: %d", rec.Code)
	}
	if rec := second.get("/accounts/profile"); rec.Code == http.StatusOK {
		t.Error("the other session should have been revoked")
	}
}

func TestTurningPasswordChangeOffHidesAndBlocksIt(t *testing.T) {
	a, c := changeApp(t, func(s *settings.Settings) { s.Auth.AllowPasswordChange = false })
	signedInAsJane(t, c)

	if a.Auth.AllowsPasswordChange() {
		t.Fatal("the setting should be off")
	}

	if body := c.get("/accounts/profile").Body.String(); strings.Contains(body, "Change password") {
		t.Error("the form should not be rendered when changes are off")
	}

	if rec := c.get("/accounts/password"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /accounts/password = %d, want 404 when the setting is off", rec.Code)
	}
	if code := changePassword(t, c, "her-first-secret", "her-second-secret", "her-second-secret"); code != http.StatusNotFound {
		t.Errorf("POST /accounts/password = %d, want 404", code)
	}
	if _, err := a.Auth.Authenticate("jane", "her-first-secret"); err != nil {
		t.Error("the password must be unchanged")
	}
}

func TestTheAdminPasswordFieldFollowsTheSetting(t *testing.T) {
	on, c := changeApp(t)
	c.login("/admin/login", "root", "unrelated-and-long")
	id := jane(t, on)

	if body := c.get("/admin/users/" + id).Body.String(); !strings.Contains(body, `name="password"`) {
		t.Error("the admin should offer a password field by default")
	}

	off, oc := changeApp(t, func(s *settings.Settings) { s.Auth.AllowPasswordChange = false })
	oc.login("/admin/login", "root", "unrelated-and-long")
	offID := jane(t, off)

	body := oc.get("/admin/users/" + offID).Body.String()
	if strings.Contains(body, `name="password"`) {
		t.Error("the field should disappear when the setting is off")
	}

	rec := oc.do(http.MethodPost, "/admin/users/"+offID, url.Values{
		"csrf_token": {oc.token("/admin/users/" + offID)},
		"username":   {"jane"},
		"is_active":  {"1"},
		"password":   {"crafted-by-hand-here"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rec.Code)
	}
	if _, err := off.Auth.Authenticate("jane", "crafted-by-hand-here"); err == nil {
		t.Error("a hand-crafted password must be ignored when changes are off")
	}
	if _, err := off.Auth.Authenticate("jane", "her-first-secret"); err != nil {
		t.Error("the original password should stand")
	}
}

func jane(t *testing.T, a *app.App) string {
	t.Helper()
	user, err := a.Auth.Users().ByUsername("jane")
	if err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func TestResetTokensAreSingleUseAndExpiring(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Auth.ResetTokens = true
		s.Auth.ResetTokenLifetime = time.Hour
	})
	user, err := a.Auth.CreateUser(auth.NewUser{Username: "jane", Password: "her-first-secret"})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := a.Auth.CreateResetToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" {
		t.Fatal("no token returned")
	}

	stored, err := a.Auth.Tokens().ByDigest(auth.TokenDigest(plain))
	if err != nil {
		t.Fatal(err)
	}
	if stored.Digest == plain {
		t.Error("the token must be hashed at rest, never stored in the clear")
	}

	if _, err := a.Auth.CheckResetToken(plain); err != nil {
		t.Errorf("a fresh token should verify: %v", err)
	}
	if _, err := a.Auth.CheckResetToken("made-up-token"); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Errorf("an unknown token = %v, want ErrTokenInvalid", err)
	}

	if _, err := a.Auth.UseResetToken(plain, auth.PasswordChange{New: "brand-new-secret", Confirm: "mismatch"}); !errors.Is(err, auth.ErrPasswordMismatch) {
		t.Errorf("a mismatch should be refused: %v", err)
	}
	if _, err := a.Auth.UseResetToken(plain, auth.PasswordChange{New: "brand-new-secret", Confirm: "brand-new-secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.Authenticate("jane", "brand-new-secret"); err != nil {
		t.Errorf("the reset should have applied: %v", err)
	}
	if _, err := a.Auth.CheckResetToken(plain); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Error("a used token must not verify a second time")
	}
}

func TestResetTokensAreOffUnlessAskedFor(t *testing.T) {
	a := newTestApp(t)
	if a.Auth.Tokens() != nil {
		t.Error("no token store should exist by default")
	}
	if _, err := a.Auth.CreateResetToken("whoever"); !errors.Is(err, auth.ErrNoTokenStore) {
		t.Errorf("error = %v, want ErrNoTokenStore", err)
	}
}
