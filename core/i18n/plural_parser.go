package i18n

import (
	"fmt"
	"strconv"
	"strings"
)

type parser struct {
	tokens []string
	at     int
}

func parseExpression(source string) (node, error) {
	tokens, err := tokenise(source)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("%w: empty expression", ErrBadPluralRule)
	}

	p := &parser{tokens: tokens}
	tree, err := p.ternary()
	if err != nil {
		return nil, err
	}
	if p.at != len(p.tokens) {
		return nil, fmt.Errorf("%w: unexpected %q in %q", ErrBadPluralRule, p.tokens[p.at], source)
	}
	return tree, nil
}

func (p *parser) peek() string {
	if p.at >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.at]
}

func (p *parser) take(candidates ...string) (string, bool) {
	next := p.peek()
	for _, candidate := range candidates {
		if next == candidate {
			p.at++
			return next, true
		}
	}
	return "", false
}

func (p *parser) ternary() (node, error) {
	condition, err := p.binaryLevel(0)
	if err != nil {
		return nil, err
	}
	if _, found := p.take("?"); !found {
		return condition, nil
	}

	yes, err := p.ternary()
	if err != nil {
		return nil, err
	}
	if _, found := p.take(":"); !found {
		return nil, fmt.Errorf("%w: expected \":\" after \"?\"", ErrBadPluralRule)
	}
	no, err := p.ternary()
	if err != nil {
		return nil, err
	}
	return ternary{condition: condition, yes: yes, no: no}, nil
}

var precedence = [][]string{
	{"||"},
	{"&&"},
	{"==", "!="},
	{"<", "<=", ">", ">="},
	{"+", "-"},
	{"*", "/", "%"},
}

func (p *parser) binaryLevel(level int) (node, error) {
	if level >= len(precedence) {
		return p.unary()
	}

	left, err := p.binaryLevel(level + 1)
	if err != nil {
		return nil, err
	}
	for {
		operator, found := p.take(precedence[level]...)
		if !found {
			return left, nil
		}
		right, err := p.binaryLevel(level + 1)
		if err != nil {
			return nil, err
		}
		left = binary{operator: operator, left: left, right: right}
	}
}

func (p *parser) unary() (node, error) {
	if operator, found := p.take("!"); found {
		operand, err := p.unary()
		if err != nil {
			return nil, err
		}
		return unary{operator: operator, operand: operand}, nil
	}
	return p.primary()
}

func (p *parser) primary() (node, error) {
	next := p.peek()
	switch {
	case next == "":
		return nil, fmt.Errorf("%w: expression ended early", ErrBadPluralRule)
	case next == "(":
		p.at++
		inner, err := p.ternary()
		if err != nil {
			return nil, err
		}
		if _, found := p.take(")"); !found {
			return nil, fmt.Errorf("%w: missing \")\"", ErrBadPluralRule)
		}
		return inner, nil
	case next == "n":
		p.at++
		return variable{}, nil
	}

	value, err := strconv.Atoi(next)
	if err != nil {
		return nil, fmt.Errorf("%w: unexpected %q", ErrBadPluralRule, next)
	}
	p.at++
	return literal{value: value}, nil
}

var operators = []string{"&&", "||", "==", "!=", "<=", ">=", "<", ">", "%", "*", "/", "+", "-", "?", ":", "(", ")", "!"}

func tokenise(source string) ([]string, error) {
	out := []string{}

	for at := 0; at < len(source); {
		char := source[at]
		switch {
		case char == ' ' || char == '\t' || char == '\n' || char == '\r':
			at++
			continue
		case char >= '0' && char <= '9':
			end := at
			for end < len(source) && source[end] >= '0' && source[end] <= '9' {
				end++
			}
			out = append(out, source[at:end])
			at = end
			continue
		case char == 'n':
			out = append(out, "n")
			at++
			continue
		}

		matched := ""
		for _, operator := range operators {
			if strings.HasPrefix(source[at:], operator) {
				matched = operator
				break
			}
		}
		if matched == "" {
			return nil, fmt.Errorf("%w: unexpected %q in %q", ErrBadPluralRule, string(char), source)
		}
		out = append(out, matched)
		at += len(matched)
	}
	return out, nil
}
