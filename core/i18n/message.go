package i18n

type message struct {
	context  string
	singular string
	plural   string
	forms    []string
	fuzzy    bool
}

func (m message) translated() bool {
	if m.fuzzy {
		return false
	}
	for _, form := range m.forms {
		if form != "" {
			return true
		}
	}
	return false
}

func (m message) form(index int) (string, bool) {
	if m.fuzzy || index < 0 || index >= len(m.forms) {
		return "", false
	}
	if m.forms[index] == "" {
		return "", false
	}
	return m.forms[index], true
}

func messageKey(context, msgid string) string {
	if context == "" {
		return msgid
	}
	return context + contextSeparator + msgid
}
