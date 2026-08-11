package settings

import (
	"net"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type Engine string

const (
	SQLite   Engine = "sqlite"
	Postgres Engine = "postgres"
	MySQL    Engine = "mysql"
)

const DefaultSQLiteName = "coyote.db"

type Database struct {
	Alias           string
	Engine          Engine
	Name            string
	Host            string
	Port            int
	User            string
	Password        string
	Options         map[string]string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (d Database) IsSQLite() bool { return d.Engine == SQLite }

func (d Database) DSN() string {
	options := d.encodedOptions()
	switch d.Engine {
	case SQLite:
		if options == "" {
			return d.Name
		}
		return d.Name + "?" + options
	case Postgres:
		u := url.URL{
			Scheme:   "postgres",
			Host:     net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
			Path:     "/" + d.Name,
			RawQuery: options,
		}
		if d.User != "" {
			u.User = url.UserPassword(d.User, d.Password)
		}
		return u.String()
	case MySQL:
		credentials := d.User
		if d.Password != "" {
			credentials += ":" + d.Password
		}
		if credentials != "" {
			credentials += "@"
		}
		dsn := credentials + "tcp(" + net.JoinHostPort(d.Host, strconv.Itoa(d.Port)) + ")/" + d.Name
		if options != "" {
			dsn += "?" + options
		}
		return dsn
	default:
		return d.Name
	}
}

func (d Database) Redacted() Database {
	copied := d
	if copied.Password != "" {
		copied.Password = "••••••"
	}
	return copied
}

func (d Database) encodedOptions() string {
	if len(d.Options) == 0 {
		return ""
	}
	keys := make([]string, 0, len(d.Options))
	for k := range d.Options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := url.Values{}
	for _, k := range keys {
		values.Set(k, d.Options[k])
	}
	return values.Encode()
}

func SQLiteDatabase(alias, name string) Database {
	if alias == "" {
		alias = "default"
	}
	if name == "" {
		name = DefaultSQLiteName
	}
	return Database{Alias: alias, Engine: SQLite, Name: name}
}
