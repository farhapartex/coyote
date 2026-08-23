package settings

import (
	"os"
	"path/filepath"
	"strconv"
)

func (s *Settings) normalize() {
	if host := os.Getenv("COYOTE_HOST"); host != "" {
		s.Server.Host = host
		s.commandOverrides = append(s.commandOverrides, "Server.Host")
	}
	if port := os.Getenv("COYOTE_PORT"); port != "" {
		if n, err := strconv.Atoi(port); err == nil {
			s.Server.Port = n
			s.commandOverrides = append(s.commandOverrides, "Server.Port")
		}
	}
	if s.BaseDir == "" {
		s.BaseDir = workingDir()
	}
	if abs, err := filepath.Abs(s.BaseDir); err == nil {
		s.BaseDir = abs
	}

	if s.Server.TLS.Autocert {
		if s.Server.TLS.CacheDir == "" {
			s.Server.TLS.CacheDir = "certs"
		}
		if !filepath.IsAbs(s.Server.TLS.CacheDir) {
			s.Server.TLS.CacheDir = filepath.Join(s.BaseDir, s.Server.TLS.CacheDir)
		}
	}

	if s.Migrations.Dir == "" {
		s.Migrations.Dir = "migrations"
	}
	if !filepath.IsAbs(s.Migrations.Dir) {
		s.Migrations.Dir = filepath.Join(s.BaseDir, s.Migrations.Dir)
	}

	s.normalizeCaches()

	for i := range s.Databases {
		db := &s.Databases[i]
		if db.Alias == "" {
			if i == 0 {
				db.Alias = defaultAlias
			} else {
				db.Alias = "db" + strconv.Itoa(i)
			}
		}
		if db.Engine == "" {
			db.Engine = SQLite
		}
		switch db.Engine {
		case SQLite:
			if db.Name == "" {
				db.Name = DefaultSQLiteName
			}
			if db.Name != ":memory:" && !filepath.IsAbs(db.Name) {
				db.Name = filepath.Join(s.BaseDir, db.Name)
			}
		case Postgres:
			if db.Host == "" {
				db.Host = "127.0.0.1"
			}
			if db.Port == 0 {
				db.Port = 5432
			}
		case MySQL:
			if db.Host == "" {
				db.Host = "127.0.0.1"
			}
			if db.Port == 0 {
				db.Port = 3306
			}
		}
	}
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
