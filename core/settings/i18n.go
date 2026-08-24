package settings

import (
	"io/fs"

	"github.com/farhapartex/coyote/core/i18n"
)

type I18N struct {
	Default    string
	Supported  []string
	Dir        string
	FS         fs.FS
	Fallbacks  map[string]string
	URLPrefix  bool
	CookieName string
	Loader     i18n.Loader
	Formatter  i18n.Formatter
}

func (i I18N) Enabled() bool { return len(i.Supported) > 1 || i.Loader != nil || i.FS != nil }

func (i I18N) Locale() string {
	if i.Default == "" {
		return i18n.DefaultTag
	}
	return i.Default
}

func (i I18N) Cookie() string {
	if i.CookieName == "" {
		return i18n.DefaultCookieName
	}
	return i.CookieName
}
