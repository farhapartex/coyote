package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
	if _, err := service.CreateUser(auth.NewUser{Username: "jane", Password: "unrelated-and-long"}); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestThrottlingIsOffByDefault(t *testing.T) {
	if settings.Default().Auth.Throttle.Enabled {
		t.Fatal("login throttling must be off unless asked for")
	}

	service := throttledService(t, auth.ThrottlePolicy{})
	for i := range 20 {
		if _, err := service.Authenticate("jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v, want ErrInvalidCredentials", i, err)
		}
	}
}

func TestLockoutAfterRepeatedFailures(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 3, Window: time.Minute, Lockout: time.Minute,
	})

	for i := range 3 {
		if _, err := service.Authenticate("jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v", i, err)
		}
	}

	if _, err := service.Authenticate("jane", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Errorf("error = %v, want ErrTooManyAttempts", err)
	}
	if _, err := service.Authenticate("jane", "unrelated-and-long"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Error("a locked account must be refused even with the right password")
	}
}

func TestSuccessClearsTheCounter(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 3, Window: time.Minute,
	})

	service.Authenticate("jane", "wrong")
	service.Authenticate("jane", "wrong")

	if _, err := service.Authenticate("jane", "unrelated-and-long"); err != nil {
		t.Fatalf("a good password below the limit should work: %v", err)
	}

	for i := range 3 {
		if _, err := service.Authenticate("jane", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("attempt %d after a reset: error = %v", i, err)
		}
	}
}

func TestLockoutExpires(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 2, Window: time.Minute, Lockout: 40 * time.Millisecond,
	})

	service.Authenticate("jane", "wrong")
	service.Authenticate("jane", "wrong")
	if _, err := service.Authenticate("jane", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Fatalf("expected a lockout, got %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	if _, err := service.Authenticate("jane", "unrelated-and-long"); err != nil {
		t.Errorf("the lockout should have expired: %v", err)
	}
}

func TestUnknownUsernamesAreThrottledToo(t *testing.T) {
	service := throttledService(t, auth.ThrottlePolicy{
		Enabled: true, MaxAttempts: 2, Window: time.Minute,
	})

	service.Authenticate("ghost", "wrong")
	service.Authenticate("ghost", "wrong")

	if _, err := service.Authenticate("ghost", "wrong"); !errors.Is(err, auth.ErrTooManyAttempts) {
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
	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "supersecret"); err != nil {
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
