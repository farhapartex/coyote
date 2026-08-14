package settings

import (
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

func (s Server) Addr() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func (s Settings) Addr() string { return s.Server.Addr() }

func (s Settings) Scheme() string { return s.Server.TLS.Scheme() }

func (s Settings) BaseURL() string { return s.Scheme() + "://" + s.Addr() }

func (s Settings) SecretKeyGenerated() bool { return s.secretKeyGenerated }

func (s Settings) CommandOverrides() []string { return s.commandOverrides }

func (s Settings) LogLevel() slog.Level {
	switch strings.ToLower(s.Logging.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (s Settings) IsDevelopment() bool { return s.Environment == Development }

func (s Settings) IsStaging() bool { return s.Environment == Staging }

func (s Settings) IsProduction() bool { return s.Environment == Production }

func (s Settings) IsDeployed() bool { return s.Environment.Deployed() }

func (s Settings) AutoReloadTemplates() bool { return s.Debug }

func (s Settings) Database() Database {
	if len(s.Databases) == 0 {
		return Database{}
	}
	return s.Databases[0]
}

func (s Settings) DatabaseByAlias(alias string) (Database, bool) {
	for _, db := range s.Databases {
		if db.Alias == alias {
			return db, true
		}
	}
	return Database{}, false
}

func (s Settings) Path(elements ...string) string {
	return filepath.Join(append([]string{s.BaseDir}, elements...)...)
}
