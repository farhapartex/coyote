package tests

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func throttledService(t *testing.T, policy auth.ThrottlePolicy) *auth.Service {
	t.Helper()
	service := auth.NewService(auth.NewMemoryStore(), newTestManager(), auth.Options{
		Hasher:            auth.Hasher{Iterations: 1000},
		MinPasswordLength: 8,
		Throttle:          policy,
	})
	if _, err := service.CreateUser(t.Context(), auth.NewUser{Username: "jane", Password: "unrelated-and-long"}); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestThrottlingIsOnByDefault(t *testing.T) {
	policy := settings.Default().Auth.Throttle
	if !policy.Enabled {
		t.Fatal("login throttling must be on without being asked for")
	}
	if policy.MaxAttempts != 5 || policy.Window != 15*time.Minute || policy.Lockout != 15*time.Minute {
		t.Errorf("policy = %+v, want 5 attempts over 15 minutes and a 15 minute lockout", policy)
	}

	service := throttledService(t, policy)
	for i := range policy.MaxAttempts {
		if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v, want ErrInvalidCredentials", i, err)
		}
	}
	if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Errorf("error = %v, want ErrTooManyAttempts once the default is spent", err)
	}
}

func TestNoPolicyAtAllStillMeansNoThrottling(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{})
	for i := range 20 {
		if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v, want ErrInvalidCredentials", i, err)
		}
	}
}

func TestTheDefaultThrottleStopsBruteForcingTheAdminLogin(t *testing.T) {
	_, c := setupAdmin(t)

	for attempt := range 5 {
		if response := c.login("/admin/login", "root", "wrong"); response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d answered %d, want 401", attempt+1, response.StatusCode)
		}
	}
	if response := c.login("/admin/login", "root", "wrong"); response.StatusCode != http.StatusTooManyRequests {
		t.Errorf("the sixth guess answered %d, want 429 without the project configuring anything",
			response.StatusCode)
	}
	if response := c.login("/admin/login", "root", "supersecret"); response.StatusCode == http.StatusSeeOther {
		t.Error("the lockout let the right password straight through")
	}
}

func TestLockoutAfterRepeatedFailures(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 3, Window: time.Minute, Lockout: time.Minute,
	})

	for i := range 3 {
		if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v", i, err)
		}
	}

	if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Errorf("error = %v, want ErrTooManyAttempts", err)
	}
	if _, err := service.Authenticate(t.Context(), "jane", "unrelated-and-long"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Error("a locked account must be refused even with the right password")
	}
}

func TestSuccessClearsTheCounter(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 3, Window: time.Minute,
	})

	service.Authenticate(t.Context(), "jane", "wrong")
	service.Authenticate(t.Context(), "jane", "wrong")

	if _, err := service.Authenticate(t.Context(), "jane", "unrelated-and-long"); err != nil {
		t.Fatalf("a good password below the limit should work: %v", err)
	}

	for i := range 3 {
		if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("attempt %d after a reset: error = %v", i, err)
		}
	}
}

func TestLockoutExpires(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 2, Window: time.Minute, Lockout: 40 * time.Millisecond,
	})

	service.Authenticate(t.Context(), "jane", "wrong")
	service.Authenticate(t.Context(), "jane", "wrong")
	if _, err := service.Authenticate(t.Context(), "jane", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Fatalf("expected a lockout, got %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	if _, err := service.Authenticate(t.Context(), "jane", "unrelated-and-long"); err != nil {
		t.Errorf("the lockout should have expired: %v", err)
	}
}

func TestUnknownUsernamesAreThrottledToo(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 2, Window: time.Minute,
	})

	service.Authenticate(t.Context(), "ghost", "wrong")
	service.Authenticate(t.Context(), "ghost", "wrong")

	if _, err := service.Authenticate(t.Context(), "ghost", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Error("an unknown username must lock out like a real one, or the lockout reveals which accounts exist")
	}
}

func TestThrottlingIsPerUsernameAndClient(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 2, Window: time.Minute,
	})

	attacker := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	attacker.RemoteAddr = "10.0.0.9:1234"
	victim := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	victim.RemoteAddr = "10.0.0.1:1234"

	service.AuthenticateRequest(attacker, "jane", "wrong")
	service.AuthenticateRequest(attacker, "jane", "wrong")
	if _, err := service.AuthenticateRequest(attacker, "jane", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Fatalf("the attacker should be locked out, got %v", err)
	}

	if _, err := service.AuthenticateRequest(victim, "jane", "unrelated-and-long"); err != nil {
		t.Errorf("the real user must not be locked out by someone else's guessing: %v", err)
	}
}

func TestAdminLoginReportsALockout(t *testing.T) {
	base := func(s *settings.Settings) {
		s.Admin.SiteName = "Test admin"
		s.Auth.Throttle = auth.ThrottlePolicy{Enabled: true, MaxAttempts: 2, Window: time.Minute}
	}
	a := newTestApp(t, base)
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "supersecret"); err != nil {
		t.Fatal(err)
	}
	admin.Mount(a)
	c := newClient(t, a.Handler())

	c.login("/admin/login", "root", "wrong")
	c.login("/admin/login", "root", "wrong")

	res := c.login("/admin/login", "root", "supersecret")
	if res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", res.StatusCode)
	}
}

func TestThrottleSettingsAreValidated(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Auth.Throttle = auth.ThrottlePolicy{Enabled: true, MaxAttempts: 0, Window: 0}
	})...)
	if err == nil {
		t.Fatal("an enabled throttle with no limits should be rejected")
	}
	problems := problemsOf(t, err)
	mustContain(t, problems, "Auth.Throttle.MaxAttempts")
	mustContain(t, problems, "Auth.Throttle.Window")
}

func TestForgedForwardingCannotEscapeTheLoginLockout(t *testing.T) {
	a, c := setupAdmin(t, func(s *settings.Settings) { s.Security.TrustedProxyCount = 1 })
	token := c.token("/admin/login")

	guess := func(forged string) int {
		form := url.Values{"csrf_token": {token}, "username": {"root"}, "password": {"wrong"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", forged+", 203.0.113.9")
		req.AddCookie(c.cookie)
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	for attempt := range 5 {
		if code := guess(fmt.Sprintf("9.9.9.%d", attempt)); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d answered %d, want 401", attempt+1, code)
		}
	}
	if code := guess("9.9.9.99"); code != http.StatusTooManyRequests {
		t.Errorf("a guess from a fresh forged address answered %d, want 429; rotating the left of the "+
			"chain must not buy a new allowance", code)
	}
}

func TestThrottlingPoolsAnIPv6NetworkIntoOneLockout(t *testing.T) {
	a, c := setupAdmin(t, func(s *settings.Settings) { s.Security.TrustedProxyCount = 1 })
	token := c.token("/admin/login")

	guess := func(client string) int {
		form := url.Values{"csrf_token": {token}, "username": {"root"}, "password": {"wrong"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", client)
		req.AddCookie(c.cookie)
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	for attempt := range 5 {
		if code := guess(fmt.Sprintf("2001:db8:1:2::%d", attempt+1)); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d answered %d, want 401", attempt+1, code)
		}
	}
	if code := guess("2001:db8:1:2::ffff"); code != http.StatusTooManyRequests {
		t.Errorf("a fresh address in the same /64 answered %d, want 429", code)
	}
	if code := guess("2001:db8:1:3::1"); code != http.StatusUnauthorized {
		t.Errorf("a different /64 answered %d, want its own allowance", code)
	}
}
