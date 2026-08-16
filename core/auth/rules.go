package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrPasswordNumericOnly = errors.New("coyote/auth: password is entirely numeric")
	ErrPasswordSimilar     = errors.New("coyote/auth: password is too similar to the account details")
	ErrPasswordCommon      = errors.New("coyote/auth: password is one of the most commonly used")
)

type PasswordRule func(password string, user *User) error

func DefaultPasswordRules(minLength int) []PasswordRule {
	return []PasswordRule{
		MinimumLength(minLength),
		NotEntirelyNumeric(),
		NotSimilarToAccount(),
		NotCommon(nil),
	}
}

func MinimumLength(minLength int) PasswordRule {
	return func(password string, _ *User) error {
		return ValidatePasswordLength(password, minLength)
	}
}

func NotEntirelyNumeric() PasswordRule {
	return func(password string, _ *User) error {
		if password == "" {
			return nil
		}
		for _, r := range password {
			if !unicode.IsDigit(r) {
				return nil
			}
		}
		return ErrPasswordNumericOnly
	}
}

func NotSimilarToAccount() PasswordRule {
	return func(password string, user *User) error {
		if user == nil || password == "" {
			return nil
		}
		folded := strings.ToLower(password)
		for _, candidate := range []string{user.Username, user.Email, user.FirstName, user.LastName} {
			candidate = strings.ToLower(strings.TrimSpace(candidate))
			if len(candidate) < 3 {
				continue
			}
			if local, _, found := strings.Cut(candidate, "@"); found {
				candidate = local
			}
			if strings.Contains(folded, candidate) || strings.Contains(candidate, folded) {
				return ErrPasswordSimilar
			}
		}
		return nil
	}
}

func NotCommon(list []string) PasswordRule {
	if list == nil {
		list = CommonPasswords()
	}
	lookup := make(map[string]struct{}, len(list))
	for _, entry := range list {
		lookup[strings.ToLower(strings.TrimSpace(entry))] = struct{}{}
	}
	return func(password string, _ *User) error {
		if _, found := lookup[strings.ToLower(password)]; found {
			return ErrPasswordCommon
		}
		return nil
	}
}

func ApplyPasswordRules(rules []PasswordRule, password string, user *User) error {
	problems := make([]error, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		if err := rule(password, user); err != nil {
			problems = append(problems, err)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrPasswordRejected, errors.Join(problems...))
}
