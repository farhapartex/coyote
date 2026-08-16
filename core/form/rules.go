package form

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

type Rule func(value string, arg string) error

var (
	rulesMu sync.RWMutex
	rules   = map[string]Rule{
		"required": required,
		"email":    email,
		"url":      link,
		"min":      min,
		"max":      max,
		"len":      length,
		"oneof":    oneOf,
		"match":    match,
		"numeric":  numeric,
		"alphanum": alphanumeric,
	}
	patterns   = map[string]*regexp.Regexp{}
	patternsMu sync.Mutex
)

func Register(name string, rule Rule) {
	rulesMu.Lock()
	defer rulesMu.Unlock()
	rules[name] = rule
}

func lookup(name string) (Rule, bool) {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	rule, found := rules[name]
	return rule, found
}

func required(value, _ string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("is required")
	}
	return nil
}

func email(value, _ string) error {
	if value == "" {
		return nil
	}
	if _, err := mail.ParseAddress(value); err != nil {
		return fmt.Errorf("is not a valid email address")
	}
	return nil
}

func link(value, _ string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("is not a valid URL")
	}
	return nil
}

func min(value, arg string) error {
	if value == "" {
		return nil
	}
	limit, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return nil
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		if number < limit {
			return fmt.Errorf("must be at least %s", arg)
		}
		return nil
	}
	if utf8.RuneCountInString(value) < int(limit) {
		return fmt.Errorf("must be at least %s characters", arg)
	}
	return nil
}

func max(value, arg string) error {
	if value == "" {
		return nil
	}
	limit, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return nil
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		if number > limit {
			return fmt.Errorf("must be at most %s", arg)
		}
		return nil
	}
	if utf8.RuneCountInString(value) > int(limit) {
		return fmt.Errorf("must be at most %s characters", arg)
	}
	return nil
}

func length(value, arg string) error {
	if value == "" {
		return nil
	}
	want, err := strconv.Atoi(arg)
	if err != nil {
		return nil
	}
	if utf8.RuneCountInString(value) != want {
		return fmt.Errorf("must be exactly %d characters", want)
	}
	return nil
}

func oneOf(value, arg string) error {
	if value == "" {
		return nil
	}
	for _, candidate := range strings.Fields(strings.ReplaceAll(arg, "|", " ")) {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("must be one of %s", strings.ReplaceAll(arg, "|", ", "))
}

func match(value, arg string) error {
	if value == "" {
		return nil
	}
	patternsMu.Lock()
	compiled, found := patterns[arg]
	if !found {
		var err error
		compiled, err = regexp.Compile(arg)
		if err != nil {
			patternsMu.Unlock()
			return nil
		}
		patterns[arg] = compiled
	}
	patternsMu.Unlock()

	if !compiled.MatchString(value) {
		return fmt.Errorf("is not in the expected format")
	}
	return nil
}

func numeric(value, _ string) error {
	if value == "" {
		return nil
	}
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return fmt.Errorf("must be a number")
	}
	return nil
}

func alphanumeric(value, _ string) error {
	if value == "" {
		return nil
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return fmt.Errorf("must contain only letters and digits")
		}
	}
	return nil
}
