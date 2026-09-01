package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func ruleService(t *testing.T, rules []auth.PasswordRule) *auth.Service {
	t.Helper()
	return auth.NewService(auth.NewMemoryStore(), newTestManager(), auth.Options{
		Hasher:            auth.Hasher{Iterations: 1000},
		MinPasswordLength: 8,
		PasswordRules:     rules,
	})
}

func TestDefaultRulesRejectShortNumericAndCommonPasswords(t *testing.T) {
	service := ruleService(t, nil)

	for _, testcase := range []struct {
		password string
		want     error
	}{
		{"short", auth.ErrPasswordTooShort},
		{"1234567890123", auth.ErrPasswordNumericOnly},
		{"password123", auth.ErrPasswordCommon},
		{"qwertyuiop", auth.ErrPasswordCommon},
	} {
		err := service.ValidatePassword(testcase.password)
		if err == nil {
			t.Errorf("%q should have been rejected", testcase.password)
			continue
		}
		if !errors.Is(err, testcase.want) {
			t.Errorf("%q: error = %v, want %v", testcase.password, err, testcase.want)
		}
		if !errors.Is(err, auth.ErrPasswordRejected) {
			t.Errorf("%q should wrap ErrPasswordRejected", testcase.password)
		}
	}

	if err := service.ValidatePassword("a-perfectly-fine-secret"); err != nil {
		t.Errorf("a good password was rejected: %v", err)
	}
}

func TestRulesReportEveryProblemAtOnce(t *testing.T) {
	service := ruleService(t, nil)

	err := service.ValidatePassword("123456")
	if err == nil {
		t.Fatal("expected a rejection")
	}
	for _, want := range []error{auth.ErrPasswordTooShort, auth.ErrPasswordNumericOnly, auth.ErrPasswordCommon} {
		if !errors.Is(err, want) {
			t.Errorf("all failing rules should be reported; missing %v in %v", want, err)
		}
	}
}

func TestPasswordCannotResembleTheAccount(t *testing.T) {
	service := ruleService(t, nil)

	_, err := service.CreateUser(t.Context(), auth.NewUser{
		Username: "jane.doe",
		Email:    "jane@example.com",
		Password: "jane.doe.jane.doe",
	})
	if !errors.Is(err, auth.ErrPasswordSimilar) {
		t.Errorf("error = %v, want ErrPasswordSimilar", err)
	}

	if _, err := service.CreateUser(t.Context(), auth.NewUser{
		Username: "jane.doe",
		Email:    "jane@example.com",
		Password: "unrelated-and-long",
	}); err != nil {
		t.Errorf("an unrelated password should be accepted: %v", err)
	}
}

func TestRulesAlsoGuardSetPassword(t *testing.T) {
	service := ruleService(t, nil)

	user, err := service.CreateUser(t.Context(), auth.NewUser{Username: "jane", Password: "unrelated-and-long"})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.SetPassword(t.Context(), user.ID, "password"); !errors.Is(err, auth.ErrPasswordCommon) {
		t.Errorf("SetPassword error = %v, want ErrPasswordCommon", err)
	}
	if err := service.SetPassword(t.Context(), user.ID, "jane-is-my-name"); !errors.Is(err, auth.ErrPasswordSimilar) {
		t.Errorf("SetPassword should compare against the stored account, got %v", err)
	}
	if err := service.SetPassword(t.Context(), user.ID, "a-different-secret"); err != nil {
		t.Errorf("a good password was rejected: %v", err)
	}
}

func TestCustomRulesReplaceTheDefaults(t *testing.T) {
	digits := func(password string, _ *auth.User) error {
		if !strings.ContainsAny(password, "0123456789") {
			return errors.New("needs a digit")
		}
		return nil
	}
	service := ruleService(t, append(auth.DefaultPasswordRules(8), digits))

	if err := service.ValidatePassword("no-digits-at-all"); err == nil {
		t.Error("a custom rule should be applied")
	}
	if err := service.ValidatePassword("has-1-digit-here"); err != nil {
		t.Errorf("a password passing every rule was rejected: %v", err)
	}

	relaxed := ruleService(t, []auth.PasswordRule{auth.MinimumLength(4)})
	if err := relaxed.ValidatePassword("1234"); err != nil {
		t.Errorf("replacing the defaults should drop them entirely: %v", err)
	}
}

func TestCommonPasswordListIsReplaceable(t *testing.T) {
	service := ruleService(t, []auth.PasswordRule{auth.NotCommon([]string{"hunter2"})})

	if err := service.ValidatePassword("hunter2"); !errors.Is(err, auth.ErrPasswordCommon) {
		t.Errorf("a supplied list should be used, got %v", err)
	}
	if err := service.ValidatePassword("password"); err != nil {
		t.Error("a supplied list should replace the built-in one")
	}
	if len(auth.CommonPasswords()) < 100 {
		t.Errorf("the built-in list has only %d entries", len(auth.CommonPasswords()))
	}
}

func TestSettingsCarryPasswordRulesIntoTheApp(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Auth.PasswordRules = []auth.PasswordRule{auth.MinimumLength(20)}
	})

	if err := a.Auth.ValidatePassword("short-ish-password"); err == nil {
		t.Error("the rules from settings should be in force")
	}
	if err := a.Auth.ValidatePassword("a-very-long-password-indeed"); err != nil {
		t.Errorf("a compliant password was rejected: %v", err)
	}
}

func TestRulesAreSkippedWhenThereAreNone(t *testing.T) {
	service := auth.NewService(auth.NewMemoryStore(), newTestManager(), auth.Options{
		Hasher:        auth.Hasher{Iterations: 1000},
		PasswordRules: []auth.PasswordRule{},
	})

	if err := service.ValidatePassword("x"); err != nil {
		t.Errorf("an empty rule set should accept anything, got %v", err)
	}
}
