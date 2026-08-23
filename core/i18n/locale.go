package i18n

import (
	"fmt"
	"strings"
)

type Locale struct {
	bundle *Bundle
	tag    string
	chain  []string
}

func (l *Locale) Tag() string {
	if l == nil {
		return DefaultTag
	}
	return l.tag
}

func (l *Locale) Base() string { return BaseOf(l.Tag()) }

func (l *Locale) Direction() string { return DirectionOf(l.Tag()) }

func (l *Locale) RTL() bool { return l.Direction() == RightToLeft }

func (l *Locale) Chain() []string {
	if l == nil {
		return []string{DefaultTag}
	}
	return append([]string{}, l.chain...)
}

func (l *Locale) T(msgid string) string {
	return l.translate("", msgid)
}

func (l *Locale) Tf(msgid string, args ...any) string {
	return interpolate(l.translate("", msgid), args...)
}

func (l *Locale) TC(context, msgid string) string {
	return l.translate(context, msgid)
}

func (l *Locale) TCf(context, msgid string, args ...any) string {
	return interpolate(l.translate(context, msgid), args...)
}

func (l *Locale) N(singular, plural string, count int, args ...any) string {
	return l.pluralise("", singular, plural, count, args...)
}

func (l *Locale) NC(context, singular, plural string, count int, args ...any) string {
	return l.pluralise(context, singular, plural, count, args...)
}

func (l *Locale) translate(context, msgid string) string {
	if l == nil || l.bundle == nil {
		return msgid
	}
	if value, found := l.bundle.lookup(l.chain, context, msgid, 1, false); found {
		return value
	}
	l.bundle.reportMissing(l.tag, msgid)
	return msgid
}

func (l *Locale) pluralise(context, singular, plural string, count int, args ...any) string {
	if l == nil || l.bundle == nil {
		return interpolate(englishPlural(singular, plural, count), prepend(count, args)...)
	}
	if value, found := l.bundle.lookup(l.chain, context, singular, count, true); found {
		return interpolate(value, prepend(count, args)...)
	}
	l.bundle.reportMissing(l.tag, singular)
	return interpolate(englishPlural(singular, plural, count), prepend(count, args)...)
}

func (l *Locale) Available() []LocaleInfo {
	if l == nil || l.bundle == nil {
		return []LocaleInfo{{Tag: DefaultTag, Direction: LeftToRight, Active: true}}
	}
	tags := l.bundle.Supported()
	out := make([]LocaleInfo, 0, len(tags))
	for _, tag := range tags {
		out = append(out, LocaleInfo{
			Tag:       tag,
			Name:      NameOf(tag),
			Direction: DirectionOf(tag),
			Active:    tag == l.tag,
		})
	}
	return out
}

func (l *Locale) Missing() []string {
	if l == nil || l.bundle == nil {
		return nil
	}
	return l.bundle.Missing()
}

type LocaleInfo struct {
	Tag       string
	Name      string
	Direction string
	Active    bool
}

func englishPlural(singular, plural string, count int) string {
	if count == 1 || plural == "" {
		return singular
	}
	return plural
}

func interpolate(text string, args ...any) string {
	if len(args) == 0 || !strings.ContainsRune(text, '%') {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func prepend(count int, args []any) []any {
	if len(args) > 0 {
		return args
	}
	return []any{count}
}
