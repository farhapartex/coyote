package settings

import (
	"strconv"
	"strings"
)

func (s Settings) validateDatabases(add func(string)) {
	if len(s.Databases) == 0 {
		add("Databases is empty; the first entry is the default connection, normally SQLite")
	}
	seenAliases := make(map[string]bool, len(s.Databases))
	for i, db := range s.Databases {
		label := "Databases[" + strconv.Itoa(i) + "]"
		if db.Alias != "" {
			if seenAliases[db.Alias] {
				add(label + " reuses the alias " + strconv.Quote(db.Alias))
			}
			seenAliases[db.Alias] = true
		}
		switch db.Engine {
		case SQLite:
			if db.Name == "" {
				add(label + ".Name is empty; it is the SQLite file path")
			}
			if db.Host != "" || db.User != "" || db.Password != "" {
				add(label + " is SQLite, so Host, User and Password are not used")
			}
		case Postgres, MySQL:
			if strings.TrimSpace(db.Name) == "" {
				add(label + ".Name is empty; it is the database name")
			}
			if strings.TrimSpace(db.Host) == "" {
				add(label + ".Host is empty")
			}
			if db.Port < 1 || db.Port > 65535 {
				add(label + ".Port must be between 1 and 65535")
			}
		case "":
			add(label + ".Engine is empty")
		default:
			add(label + ".Engine " + strconv.Quote(string(db.Engine)) + " is not supported; use \"sqlite\", \"postgres\" or \"mysql\"")
		}
		if db.MaxOpenConns < 0 {
			add(label + ".MaxOpenConns cannot be negative")
		}
		if db.MaxIdleConns < 0 {
			add(label + ".MaxIdleConns cannot be negative")
		}
		if db.ConnMaxLifetime < 0 {
			add(label + ".ConnMaxLifetime cannot be negative")
		}
		if db.ConnMaxIdleTime < 0 {
			add(label + ".ConnMaxIdleTime cannot be negative")
		}
	}
}
