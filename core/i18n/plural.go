package i18n

import (
	"fmt"
	"strconv"
	"strings"
)

type PluralRule struct {
	Count int
	tree  node
	raw   string
}

func (r PluralRule) Forms() int {
	if r.Count < 1 {
		return 1
	}
	return r.Count
}

func (r PluralRule) Source() string { return r.raw }

func (r PluralRule) Form(n int) int {
	if r.tree == nil {
		if n != 1 {
			return 1
		}
		return 0
	}
	form := r.tree.eval(n)
	if form < 0 || form >= r.Forms() {
		return 0
	}
	return form
}

func DefaultPluralRule() PluralRule {
	return PluralRule{Count: 2}
}

func ParsePluralForms(header string) (PluralRule, error) {
	rule := PluralRule{Count: 2, raw: strings.TrimSpace(header)}
	if rule.raw == "" {
		return rule, nil
	}

	expression := ""
	for _, part := range strings.Split(rule.raw, ";") {
		name, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		switch name {
		case "nplurals":
			count, err := strconv.Atoi(value)
			if err != nil || count < 1 {
				return DefaultPluralRule(), fmt.Errorf("%w: nplurals %q", ErrBadPluralRule, value)
			}
			rule.Count = count
		case "plural":
			expression = value
		default:
			if strings.HasPrefix(name, "plural") {
				expression = strings.TrimSpace(part[strings.Index(part, "=")+1:])
			}
		}
	}
	if expression == "" {
		return rule, nil
	}

	tree, err := parseExpression(expression)
	if err != nil {
		return PluralRule{Count: rule.Count, raw: rule.raw}, err
	}
	rule.tree = tree
	return rule, nil
}
