package app

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/farhapartex/coyote/core/settings"
)

func newLogger(s Settings) *slog.Logger {
	opts := &slog.HandlerOptions{Level: s.LogLevel()}
	if strings.EqualFold(s.Logging.Format, "json") {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func templateFS(s Settings) fs.FS {
	if s.Templates.FS != nil {
		return s.Templates.FS
	}
	if s.Templates.Dir != "" {
		return os.DirFS(s.Templates.Dir)
	}
	return nil
}

func staticFS(s Settings) fs.FS {
	if s.Static.FS != nil {
		return s.Static.FS
	}
	if s.Static.Dir != "" {
		return os.DirFS(s.Static.Dir)
	}
	return nil
}

func sameSite(mode settings.SameSite) http.SameSite {
	switch mode {
	case settings.SameSiteStrict:
		return http.SameSiteStrictMode
	case settings.SameSiteNone:
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
