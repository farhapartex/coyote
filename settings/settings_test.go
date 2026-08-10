package settings

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func production(fns ...func(*Settings)) []func(*Settings) {
	base := func(s *Settings) {
		s.Debug = false
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"example.com"}
	}
	return append([]func(*Settings){base}, fns...)
}

func problemsOf(t *testing.T, err error) []string {
	t.Helper()
	var ic *ImproperlyConfigured
	if !errors.As(err, &ic) {
		t.Fatalf("error is not *ImproperlyConfigured: %v", err)
	}
	return ic.Problems
}

func mustContain(t *testing.T, problems []string, substr string) {
	t.Helper()
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return
		}
	}
	t.Errorf("no problem mentioning %q in %v", substr, problems)
}

func TestDefaultsAreUsable(t *testing.T) {
	s, err := New(production()...)
	if err != nil {
		t.Fatalf("production defaults should validate: %v", err)
	}
	if s.Addr() != "127.0.0.1:8000" {
		t.Errorf("Addr = %q", s.Addr())
	}
	if s.Sessions.CookieName != "coyote_session" {
		t.Errorf("CookieName = %q", s.Sessions.CookieName)
	}
	if !s.Sessions.HTTPOnly {
		t.Error("HTTPOnly should default to true")
	}
	if s.Sessions.SameSite != SameSiteLax {
		t.Errorf("SameSite = %q", s.Sessions.SameSite)
	}
	if s.Sessions.Lifetime != 12*time.Hour {
		t.Errorf("Lifetime = %v", s.Sessions.Lifetime)
	}
	if s.Auth.PasswordMinLength != 8 || s.Auth.PBKDF2Iterations != 600000 {
		t.Errorf("auth defaults = %+v", s.Auth)
	}
	if s.Admin.Prefix != "/admin" {
		t.Errorf("Admin.Prefix = %q", s.Admin.Prefix)
	}
	if s.AutoReloadTemplates() {
		t.Error("template reload should be off when Debug is false")
	}
}

func TestOverridesKeepOtherDefaults(t *testing.T) {
	s, err := New(production(func(s *Settings) {
		s.Server.Port = 9999
		s.Sessions.Rolling = true
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if s.Server.Port != 9999 || !s.Sessions.Rolling {
		t.Errorf("overrides not applied: %+v", s.Server)
	}
	if s.Sessions.CookieName != "coyote_session" || s.Static.URL != "/static/" {
		t.Error("untouched defaults were lost")
	}
}

func TestSecretKeyRequiredWithoutDebug(t *testing.T) {
	_, err := New(func(s *Settings) {
		s.Debug = false
		s.AllowedHosts = []string{"example.com"}
	})
	if err == nil {
		t.Fatal("expected an error for a missing SecretKey")
	}
	mustContain(t, problemsOf(t, err), "SecretKey is empty")
}

func TestShortSecretKeyRejectedWithoutDebug(t *testing.T) {
	_, err := New(production(func(s *Settings) { s.SecretKey = "tooshort" })...)
	if err == nil {
		t.Fatal("expected an error for a short SecretKey")
	}
	mustContain(t, problemsOf(t, err), "shorter than 32")
}

func TestDebugGeneratesEphemeralSecretKey(t *testing.T) {
	s, err := New(func(s *Settings) { s.Debug = true })
	if err != nil {
		t.Fatal(err)
	}
	if s.SecretKey == "" {
		t.Fatal("expected a generated key")
	}
	if !s.SecretKeyGenerated() {
		t.Error("SecretKeyGenerated should report true")
	}
	if !s.AutoReloadTemplates() {
		t.Error("template reload should follow Debug")
	}

	other, err := New(func(s *Settings) { s.Debug = true })
	if err != nil {
		t.Fatal(err)
	}
	if other.SecretKey == s.SecretKey {
		t.Error("generated keys should differ between runs")
	}
}

func TestAllowedHostsRequiredWithoutDebug(t *testing.T) {
	_, err := New(func(s *Settings) {
		s.SecretKey = strings.Repeat("k", 48)
	})
	if err == nil {
		t.Fatal("expected an error for empty AllowedHosts")
	}
	mustContain(t, problemsOf(t, err), "AllowedHosts is empty")
}

func TestValidationCollectsEveryProblem(t *testing.T) {
	_, err := New(production(func(s *Settings) {
		s.Server.Port = 0
		s.Sessions.CookieName = ""
		s.Sessions.Lifetime = 0
		s.Sessions.SameSite = "sideways"
		s.Auth.PasswordMinLength = 2
		s.Auth.PBKDF2Iterations = 10
		s.Static.URL = "static"
		s.Admin.Prefix = "admin/"
		s.Logging.Level = "loud"
		s.Logging.Format = "xml"
	})...)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	problems := problemsOf(t, err)
	for _, want := range []string{
		"Server.Port", "Sessions.CookieName", "Sessions.Lifetime", "Sessions.SameSite",
		"Auth.PasswordMinLength", "Auth.PBKDF2Iterations", "Static.URL", "Admin.Prefix",
		"Logging.Level", "Logging.Format",
	} {
		mustContain(t, problems, want)
	}
	if !strings.Contains(err.Error(), "improperly configured") {
		t.Errorf("error text = %q", err.Error())
	}
}

func TestSameSiteNoneRequiresSecure(t *testing.T) {
	_, err := New(production(func(s *Settings) { s.Sessions.SameSite = SameSiteNone })...)
	if err == nil {
		t.Fatal("expected an error for SameSite=none without Secure")
	}
	mustContain(t, problemsOf(t, err), "Sessions.Secure")

	if _, err := New(production(func(s *Settings) {
		s.Sessions.SameSite = SameSiteNone
		s.Sessions.Secure = true
	})...); err != nil {
		t.Errorf("SameSite=none with Secure should be valid: %v", err)
	}
}

func TestAdminPrefixCannotBeRoot(t *testing.T) {
	_, err := New(production(func(s *Settings) { s.Admin.Prefix = "/" })...)
	if err == nil {
		t.Fatal("expected an error for Admin.Prefix = /")
	}
	mustContain(t, problemsOf(t, err), "every route")
}

func TestTemplatesFSAndDirAreExclusive(t *testing.T) {
	_, err := New(production(func(s *Settings) {
		s.Templates.Dir = "templates"
		s.Templates.FS = fstest.MapFS{}
	})...)
	if err == nil {
		t.Fatal("expected an error when both FS and Dir are set")
	}
	mustContain(t, problemsOf(t, err), "not both")
}

func TestGetPanicsUntilConfigured(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	if IsConfigured() {
		t.Fatal("should start unconfigured")
	}

	func() {
		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatal("Get should panic before Configure")
			}
			if !strings.Contains(fmtPanic(recovered), "settings.go") {
				t.Errorf("panic should point at settings.go, got %v", recovered)
			}
		}()
		Get()
	}()

	Configure(func(s *Settings) {
		s.Debug = true
		s.Server.Port = 4321
	})
	if !IsConfigured() {
		t.Fatal("should be configured now")
	}
	if Get().Server.Port != 4321 {
		t.Errorf("Get returned %d", Get().Server.Port)
	}
}

func TestConfigureTwicePanics(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Configure(func(s *Settings) { s.Debug = true })
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("second Configure should panic")
		}
	}()
	Configure(func(s *Settings) { s.Debug = true })
}

func TestConfigurePanicsOnInvalidSettings(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("Configure should panic on invalid settings")
		}
		if !strings.Contains(fmtPanic(recovered), "SecretKey") {
			t.Errorf("panic should mention SecretKey, got %v", recovered)
		}
	}()
	Configure(func(s *Settings) { s.Debug = false })
}

func TestEnvHelpers(t *testing.T) {
	t.Setenv("COYOTE_STR", "value")
	t.Setenv("COYOTE_BOOL", "yes")
	t.Setenv("COYOTE_INT", "42")
	t.Setenv("COYOTE_DUR", "90s")
	t.Setenv("COYOTE_LIST", " a , b ,, c ")
	t.Setenv("COYOTE_EMPTY", "")

	if got := Env("COYOTE_STR", "fallback"); got != "value" {
		t.Errorf("Env = %q", got)
	}
	if got := Env("COYOTE_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("empty env should fall back, got %q", got)
	}
	if got := Env("COYOTE_MISSING", "fallback"); got != "fallback" {
		t.Errorf("Env = %q", got)
	}
	if !EnvBool("COYOTE_BOOL", false) {
		t.Error("EnvBool should read yes as true")
	}
	if !EnvBool("COYOTE_MISSING", true) {
		t.Error("EnvBool should fall back")
	}
	if got := EnvInt("COYOTE_INT", 1); got != 42 {
		t.Errorf("EnvInt = %d", got)
	}
	if got := EnvInt("COYOTE_STR", 7); got != 7 {
		t.Errorf("unparsable EnvInt should fall back, got %d", got)
	}
	if got := EnvDuration("COYOTE_DUR", time.Second); got != 90*time.Second {
		t.Errorf("EnvDuration = %v", got)
	}
	if got := EnvList("COYOTE_LIST", nil); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("EnvList = %#v", got)
	}
	if got := EnvList("COYOTE_MISSING", []string{"d"}); len(got) != 1 || got[0] != "d" {
		t.Errorf("EnvList fallback = %#v", got)
	}
}

func TestGenerateSecretKeyIsLongAndUnique(t *testing.T) {
	a, err := GenerateSecretKey()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateSecretKey()
	if len(a) < minSecretKeyLength {
		t.Errorf("generated key is only %d characters", len(a))
	}
	if a == b {
		t.Error("generated keys should be unique")
	}
}

func fmtPanic(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
