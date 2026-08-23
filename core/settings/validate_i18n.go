package settings

import (
	"strconv"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/i18n"
)

func (s Settings) validateI18N(add func(string)) {
	seen := map[string]bool{}
	for i, tag := range s.I18N.Supported {
		if strings.TrimSpace(tag) == "" {
			add("I18N.Supported[" + strconv.Itoa(i) + "] is empty")
			continue
		}
		normalised := i18n.Normalise(tag)
		if seen[normalised] {
			add("I18N.Supported lists " + strconv.Quote(tag) + " more than once")
		}
		seen[normalised] = true
	}

	if s.I18N.Default != "" && len(s.I18N.Supported) > 0 && !seen[i18n.Normalise(s.I18N.Default)] {
		add("I18N.Default " + strconv.Quote(s.I18N.Default) + " is not in I18N.Supported")
	}
	for from, to := range s.I18N.Fallbacks {
		if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
			add("I18N.Fallbacks has an empty tag")
			continue
		}
		if i18n.Normalise(from) == i18n.Normalise(to) {
			add("I18N.Fallbacks maps " + strconv.Quote(from) + " to itself")
		}
	}

	if s.I18N.FS != nil && s.I18N.Dir != "" {
		add("set either I18N.FS or I18N.Dir, not both")
	}
	if s.I18N.CookieName != "" && strings.ContainsAny(s.I18N.CookieName, " ;,\t\n") {
		add("I18N.CookieName contains characters that are not valid in a cookie name")
	}

	s.validateTimeZone(add)
}

func (s Settings) validateTimeZone(add func(string)) {
	if s.TimeZone == "" {
		return
	}
	if _, err := time.LoadLocation(s.TimeZone); err != nil {
		add("TimeZone " + strconv.Quote(s.TimeZone) +
			" cannot be loaded: " + err.Error() +
			"; on a scratch image add: import _ \"time/tzdata\"")
	}
}
