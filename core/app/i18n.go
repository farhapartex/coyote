package app

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/farhapartex/coyote/core/i18n"
)

func buildBundle(s Settings, logger *slog.Logger) *i18n.Bundle {
	bundle := i18n.NewBundle(i18n.Options{
		Default:   s.I18N.Locale(),
		Supported: s.I18N.Supported,
		Fallbacks: s.I18N.Fallbacks,
		Debug:     s.Debug,
		Zone:      loadZone(s, logger),
		Formatter: s.I18N.Formatter,
		OnMissing: func(tag, msgid string) {
			if s.Debug {
				logger.Warn("no translation", "locale", tag, "message", msgid)
			}
		},
	})

	loader := localeLoader(s)
	if loader == nil {
		return bundle
	}
	for _, problem := range i18n.LoadInto(bundle, loader) {
		logger.Warn("catalog not loaded, falling back to the source text", "error", problem)
	}
	return bundle
}

func loadZone(s Settings, logger *slog.Logger) *time.Location {
	if s.TimeZone == "" {
		return time.UTC
	}
	zone, err := time.LoadLocation(s.TimeZone)
	if err != nil {
		logger.Error("time zone could not be loaded, falling back to UTC",
			"zone", s.TimeZone, "error", err)
		return time.UTC
	}
	return zone
}

func localeLoader(s Settings) i18n.Loader {
	if s.I18N.Loader != nil {
		return s.I18N.Loader
	}
	if s.I18N.FS != nil {
		return i18n.NewFSLoader(s.I18N.FS, "")
	}
	if s.I18N.Dir == "" {
		return nil
	}

	dir := s.I18N.Dir
	if !filepathIsAbs(dir) {
		dir = s.Path(dir)
	}
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return i18n.NewFSLoader(os.DirFS(dir), "")
}

func filepathIsAbs(path string) bool {
	return len(path) > 0 && (path[0] == '/' || path[0] == os.PathSeparator)
}

func (a *App) Bundle() *i18n.Bundle { return a.bundle }

func (a *App) Locale(r *http.Request) *i18n.Locale {
	if locale := i18n.FromRequest(r); locale != nil {
		return locale
	}
	if a.bundle == nil {
		return nil
	}
	return a.bundle.Locale("")
}

func (a *App) Locales() *i18n.Detector { return a.locales }

func (a *App) localeFS(s Settings) fs.FS {
	if s.I18N.FS != nil {
		return s.I18N.FS
	}
	return nil
}
