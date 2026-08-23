package i18n

import "strings"

type catalogue struct {
	tag      string
	messages map[string]message
	headers  map[string]string
	rule     PluralRule
}

func newCatalogue(tag string) *catalogue {
	return &catalogue{
		tag:      Normalise(tag),
		messages: map[string]message{},
		headers:  map[string]string{},
		rule:     DefaultPluralRule(),
	}
}

func (c *catalogue) Tag() string { return c.tag }

func (c *catalogue) Plurals() int { return c.rule.Forms() }

func (c *catalogue) Rule() PluralRule { return c.rule }

func (c *catalogue) Header(name string) string {
	return c.headers[strings.ToLower(name)]
}

func (c *catalogue) Lookup(context, msgid string) (string, bool) {
	entry, found := c.messages[messageKey(context, msgid)]
	if !found {
		return "", false
	}
	return entry.form(0)
}

func (c *catalogue) LookupPlural(context, msgid string, n int) (string, bool) {
	entry, found := c.messages[messageKey(context, msgid)]
	if !found {
		return "", false
	}
	return entry.form(c.rule.Form(n))
}

func (c *catalogue) Len() int { return len(c.messages) }

func (c *catalogue) Keys() []string {
	out := make([]string, 0, len(c.messages))
	for key := range c.messages {
		out = append(out, key)
	}
	return out
}

func (c *catalogue) add(entry message) {
	if entry.singular == "" && entry.context == "" {
		c.readHeader(entry)
		return
	}
	c.messages[messageKey(entry.context, entry.singular)] = entry
}

func (c *catalogue) readHeader(entry message) {
	if len(entry.forms) == 0 {
		return
	}
	for _, line := range strings.Split(entry.forms[0], "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		c.headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}

	if forms := c.headers["plural-forms"]; forms != "" {
		if rule, err := ParsePluralForms(forms); err == nil {
			c.rule = rule
		} else {
			c.rule = DefaultPluralRule()
			c.headers["plural-forms-error"] = err.Error()
		}
	}
	if language := c.headers["language"]; language != "" && c.tag == "" {
		c.tag = Normalise(language)
	}
}
