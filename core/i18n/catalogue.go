package i18n

import "strings"

type catalogue struct {
	tag      string
	messages map[string]message
	order    []string
	header   message
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
		c.header = entry
		c.readHeader(entry)
		return
	}
	key := messageKey(entry.context, entry.singular)
	if _, seen := c.messages[key]; !seen {
		c.order = append(c.order, key)
	}
	c.messages[key] = entry
}

func (c *catalogue) Entries() []Entry {
	out := make([]Entry, 0, len(c.order)+1)
	if len(c.header.forms) > 0 {
		out = append(out, Entry{Forms: c.header.forms, Header: true})
	}
	for _, key := range c.order {
		entry := c.messages[key]
		out = append(out, Entry{
			Context:  entry.context,
			Singular: entry.singular,
			Plural:   entry.plural,
			Forms:    entry.forms,
			Fuzzy:    entry.fuzzy,
		})
	}
	return out
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
