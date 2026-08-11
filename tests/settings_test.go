package tests

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/farhapartex/coyote/core/settings"
)

func problemsOf(t *testing.T, err error) []string {
	t.Helper()
	var ic *settings.ImproperlyConfigured
	if !errors.As(err, &ic) {
		t.Fatalf("error is not *settings.ImproperlyConfigured: %v", err)
	}
	return ic.Problems
}

func mustContain(t *testing.T, problems []string, substr string) {
	t.Helper()
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return
		}
	}
	t.Errorf("no problem mentioning %q in %v", substr, problems)
}

func TestDefaultsAreUsable(t *testing.T) {
	s, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatalf("production defaults should validate: %v", err)
	}
	if s.Addr() != "127.0.0.1:8000" {
		t.Errorf("Addr = %q", s.Addr())
	}
	if s.Sessions.CookieName != "coyote_session" {
		t.Errorf("CookieName = %q", s.Sessions.CookieName)
	}
	if !s.Sessions.HTTPOnly {
		t.Error("HTTPOnly should default to true")
	}
	if s.Sessions.SameSite != settings.SameSiteLax {
		t.Errorf("SameSite = %q", s.Sessions.SameSite)
	}
	if s.Sessions.Lifetime != 12*time.Hour {
		t.Errorf("Lifetime = %v", s.Sessions.Lifetime)
	}
	if s.Auth.PasswordMinLength != 8 || s.Auth.PBKDF2Iterations != 600000 {
		t.Errorf("auth defaults = %+v", s.Auth)
	}
	if s.Admin.Prefix != "/admin" {
		t.Errorf("Admin.Prefix = %q", s.Admin.Prefix)
	}
	if s.AutoReloadTemplates() {
		t.Error("template reload should be off when Debug is false")
	}
}

func TestDefaultDatabaseIsSQLiteInProjectFolder(t *testing.T) {
	dir := t.TempDir()
	s, err := settings.New(prodSettings(func(s *settings.Settings) { s.BaseDir = dir })...)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Databases) != 1 {
		t.Fatalf("expected one default database, got %d", len(s.Databases))
	}
	db := s.Database()
	if db.Alias != "default" {
		t.Errorf("Alias = %q, want default", db.Alias)
	}
	if db.Engine != settings.SQLite {
		t.Errorf("settings.Engine = %q, want sqlite", db.Engine)
	}
	if want := filepath.Join(dir, settings.DefaultSQLiteName); db.Name != want {
		t.Errorf("Name = %q, want %q", db.Name, want)
	}
	if !filepath.IsAbs(db.Name) {
		t.Errorf("settings.SQLite path should be absolute, got %q", db.Name)
	}
	if db.DSN() != db.Name {
		t.Errorf("settings.SQLite DSN = %q, want the file path", db.DSN())
	}
	if !db.IsSQLite() {
		t.Error("IsSQLite should be true")
	}
}

func TestBaseDirDefaultsToWorkingDirectory(t *testing.T) {
	s, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if s.BaseDir != wd {
		t.Errorf("BaseDir = %q, want %q", s.BaseDir, wd)
	}
	if got := s.Path("templates", "base.html"); got != filepath.Join(wd, "templates", "base.html") {
		t.Errorf("Path = %q", got)
	}
}

func TestRelativeSQLitePathResolvesAgainstBaseDir(t *testing.T) {
	dir := t.TempDir()
	s, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.BaseDir = dir
		s.Databases = []settings.Database{{Engine: settings.SQLite, Name: "data/app.db"}}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "data", "app.db"); s.Database().Name != want {
		t.Errorf("Name = %q, want %q", s.Database().Name, want)
	}
}

func TestAbsoluteAndMemorySQLitePathsAreLeftAlone(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "explicit.db")
	s, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Databases = []settings.Database{
			{Engine: settings.SQLite, Name: absolute},
			{Alias: "cache", Engine: settings.SQLite, Name: ":memory:"},
		}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if s.Databases[0].Name != absolute {
		t.Errorf("absolute path was rewritten to %q", s.Databases[0].Name)
	}
	if s.Databases[1].Name != ":memory:" {
		t.Errorf(":memory: was rewritten to %q", s.Databases[1].Name)
	}
}

func TestFirstDatabaseIsTheDefaultConnection(t *testing.T) {
	s, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Databases = []settings.Database{
			{Engine: settings.SQLite, Name: "primary.db"},
			{Engine: settings.Postgres, Name: "reports", Host: "db.example.com", User: "reader", Password: "pw"},
			{Engine: settings.SQLite, Name: "cache.db"},
		}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if s.Database().Alias != "default" || !strings.HasSuffix(s.Database().Name, "primary.db") {
		t.Errorf("first entry should be the default connection, got %+v", s.Database())
	}
	if s.Databases[1].Alias != "db1" || s.Databases[2].Alias != "db2" {
		t.Errorf("blank aliases should be auto-numbered, got %q and %q",
			s.Databases[1].Alias, s.Databases[2].Alias)
	}
	reports, ok := s.DatabaseByAlias("db1")
	if !ok {
		t.Fatal("DatabaseByAlias could not find db1")
	}
	if reports.Port != 5432 {
		t.Errorf("settings.Postgres port should default to 5432, got %d", reports.Port)
	}
	if _, ok := s.DatabaseByAlias("nope"); ok {
		t.Error("DatabaseByAlias should report missing aliases")
	}
}

func TestNamedAliasesArePreserved(t *testing.T) {
	s, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Databases = []settings.Database{
			{Alias: "primary", Engine: settings.SQLite, Name: "a.db"},
			{Alias: "analytics", Engine: settings.MySQL, Name: "stats", Host: "127.0.0.1"},
		}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if s.Database().Alias != "primary" {
		t.Errorf("explicit alias was overwritten: %q", s.Database().Alias)
	}
	if s.Databases[1].Port != 3306 {
		t.Errorf("settings.MySQL port should default to 3306, got %d", s.Databases[1].Port)
	}
}

func TestDatabaseDSNs(t *testing.T) {
	s, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Databases = []settings.Database{
			{Engine: settings.SQLite, Name: "/tmp/app.db", Options: map[string]string{"_pragma": "busy_timeout(5000)"}},
			{Alias: "pg", Engine: settings.Postgres, Name: "shop", Host: "db.example.com", Port: 6543,
				User: "app", Password: "s3cret", Options: map[string]string{"sslmode": "require"}},
			{Alias: "my", Engine: settings.MySQL, Name: "shop", Host: "127.0.0.1",
				User: "app", Password: "s3cret", Options: map[string]string{"parseTime": "true"}},
		}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/tmp/app.db?_pragma=busy_timeout%285000%29",
		"postgres://app:s3cret@db.example.com:6543/shop?sslmode=require",
		"app:s3cret@tcp(127.0.0.1:3306)/shop?parseTime=true",
	}
	for i, expected := range want {
		if got := s.Databases[i].DSN(); got != expected {
			t.Errorf("Databases[%d].DSN() = %q, want %q", i, got, expected)
		}
	}
}

func TestRedactedHidesPassword(t *testing.T) {
	db := settings.Database{Engine: settings.Postgres, Name: "shop", Host: "h", Port: 5432, User: "app", Password: "s3cret"}
	redacted := db.Redacted()
	if strings.Contains(redacted.Password, "s3cret") {
		t.Error("password survived redaction")
	}
	if redacted.User != "app" || redacted.Name != "shop" {
		t.Error("redaction should only touch the password")
	}
	if db.Password != "s3cret" {
		t.Error("Redacted must not mutate the original")
	}
}

func TestDatabaseValidation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*settings.Settings)
		problem string
	}{
		{"empty list", func(s *settings.Settings) { s.Databases = nil }, "Databases is empty"},
		{"unknown engine", func(s *settings.Settings) {
			s.Databases = []settings.Database{{Engine: "mongo", Name: "x"}}
		}, "not supported"},
		{"sqlite without name", func(s *settings.Settings) {
			s.Databases = []settings.Database{{Alias: "default", Engine: settings.SQLite, Name: ":memory:"},
				{Alias: "two", Engine: settings.Postgres, Host: "h"}}
		}, "Databases[1].Name is empty"},
		{"postgres with blank host", func(s *settings.Settings) {
			s.Databases = []settings.Database{{Engine: settings.Postgres, Name: "db", Host: "   "}}
		}, "Host is empty"},
		{"duplicate alias", func(s *settings.Settings) {
			s.Databases = []settings.Database{
				{Alias: "main", Engine: settings.SQLite, Name: "a.db"},
				{Alias: "main", Engine: settings.SQLite, Name: "b.db"},
			}
		}, "reuses the alias"},
		{"sqlite with credentials", func(s *settings.Settings) {
			s.Databases = []settings.Database{{Engine: settings.SQLite, Name: "a.db", User: "root"}}
		}, "are not used"},
		{"negative pool", func(s *settings.Settings) {
			s.Databases = []settings.Database{{Engine: settings.SQLite, Name: "a.db", MaxOpenConns: -1}}
		}, "MaxOpenConns cannot be negative"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := settings.New(prodSettings(c.mutate)...)
			if err == nil {
				t.Fatalf("expected a validation error")
			}
			mustContain(t, problemsOf(t, err), c.problem)
		})
	}
}

func TestSQLiteDatabaseHelper(t *testing.T) {
	db := settings.SQLiteDatabase("", "")
	if db.Alias != "default" || db.Engine != settings.SQLite || db.Name != settings.DefaultSQLiteName {
		t.Errorf("unexpected helper result: %+v", db)
	}
	custom := settings.SQLiteDatabase("cache", "cache.db")
	if custom.Alias != "cache" || custom.Name != "cache.db" {
		t.Errorf("unexpected helper result: %+v", custom)
	}
}

func TestOverridesKeepOtherDefaults(t *testing.T) {
	s, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Server.Port = 9999
		s.Sessions.Rolling = true
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if s.Server.Port != 9999 || !s.Sessions.Rolling {
		t.Errorf("overrides not applied: %+v", s.Server)
	}
	if s.Sessions.CookieName != "coyote_session" || s.Static.URL != "/static/" {
		t.Error("untouched defaults were lost")
	}
}

func TestSecretKeyRequiredWithoutDebug(t *testing.T) {
	_, err := settings.New(func(s *settings.Settings) {
		s.Debug = false
		s.AllowedHosts = []string{"example.com"}
	})
	if err == nil {
		t.Fatal("expected an error for a missing SecretKey")
	}
	mustContain(t, problemsOf(t, err), "SecretKey is empty")
}

func TestShortSecretKeyRejectedWithoutDebug(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) { s.SecretKey = "tooshort" })...)
	if err == nil {
		t.Fatal("expected an error for a short SecretKey")
	}
	mustContain(t, problemsOf(t, err), "shorter than 32")
}

func TestDebugGeneratesEphemeralSecretKey(t *testing.T) {
	s, err := settings.New(func(s *settings.Settings) { s.Debug = true })
	if err != nil {
		t.Fatal(err)
	}
	if s.SecretKey == "" {
		t.Fatal("expected a generated key")
	}
	if !s.SecretKeyGenerated() {
		t.Error("SecretKeyGenerated should report true")
	}
	if !s.AutoReloadTemplates() {
		t.Error("template reload should follow Debug")
	}

	other, err := settings.New(func(s *settings.Settings) { s.Debug = true })
	if err != nil {
		t.Fatal(err)
	}
	if other.SecretKey == s.SecretKey {
		t.Error("generated keys should differ between runs")
	}
}

func TestAllowedHostsRequiredWithoutDebug(t *testing.T) {
	_, err := settings.New(func(s *settings.Settings) {
		s.SecretKey = strings.Repeat("k", 48)
	})
	if err == nil {
		t.Fatal("expected an error for empty AllowedHosts")
	}
	mustContain(t, problemsOf(t, err), "AllowedHosts is empty")
}

func TestValidationCollectsEveryProblem(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Server.Port = 0
		s.Sessions.CookieName = ""
		s.Sessions.Lifetime = 0
		s.Sessions.SameSite = "sideways"
		s.Auth.PasswordMinLength = 2
		s.Auth.PBKDF2Iterations = 10
		s.Static.URL = "static"
		s.Admin.Prefix = "admin/"
		s.Logging.Level = "loud"
		s.Logging.Format = "xml"
	})...)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	problems := problemsOf(t, err)
	for _, want := range []string{
		"Server.Port", "Sessions.CookieName", "Sessions.Lifetime", "Sessions.SameSite",
		"Auth.PasswordMinLength", "Auth.PBKDF2Iterations", "Static.URL", "Admin.Prefix",
		"Logging.Level", "Logging.Format",
	} {
		mustContain(t, problems, want)
	}
	if !strings.Contains(err.Error(), "improperly configured") {
		t.Errorf("error text = %q", err.Error())
	}
}

func TestSameSiteNoneRequiresSecure(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) { s.Sessions.SameSite = settings.SameSiteNone })...)
	if err == nil {
		t.Fatal("expected an error for SameSite=none without Secure")
	}
	mustContain(t, problemsOf(t, err), "Sessions.Secure")

	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Sessions.SameSite = settings.SameSiteNone
		s.Sessions.Secure = true
	})...); err != nil {
		t.Errorf("SameSite=none with Secure should be valid: %v", err)
	}
}

func TestAdminPrefixCannotBeRoot(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) { s.Admin.Prefix = "/" })...)
	if err == nil {
		t.Fatal("expected an error for Admin.Prefix = /")
	}
	mustContain(t, problemsOf(t, err), "every route")
}

func TestTemplatesFSAndDirAreExclusive(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Templates.Dir = "templates"
		s.Templates.FS = fstest.MapFS{}
	})...)
	if err == nil {
		t.Fatal("expected an error when both FS and Dir are set")
	}
	mustContain(t, problemsOf(t, err), "not both")
}

func TestGetPanicsUntilConfigured(t *testing.T) {
	settings.Reset()
	t.Cleanup(settings.Reset)

	if settings.IsConfigured() {
		t.Fatal("should start unconfigured")
	}

	func() {
		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatal("settings.Get should panic before settings.Configure")
			}
			if !strings.Contains(panicText(recovered), "settings.go") {
				t.Errorf("panic should point at settings.go, got %v", recovered)
			}
		}()
		settings.Get()
	}()

	settings.Configure(func(s *settings.Settings) {
		s.Debug = true
		s.Server.Port = 4321
	})
	if !settings.IsConfigured() {
		t.Fatal("should be configured now")
	}
	if settings.Get().Server.Port != 4321 {
		t.Errorf("settings.Get returned %d", settings.Get().Server.Port)
	}
}

func TestConfigureTwicePanics(t *testing.T) {
	settings.Reset()
	t.Cleanup(settings.Reset)

	settings.Configure(func(s *settings.Settings) { s.Debug = true })
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("second settings.Configure should panic")
		}
	}()
	settings.Configure(func(s *settings.Settings) { s.Debug = true })
}

func TestConfigurePanicsOnInvalidSettings(t *testing.T) {
	settings.Reset()
	t.Cleanup(settings.Reset)

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("settings.Configure should panic on invalid settings")
		}
		if !strings.Contains(panicText(recovered), "SecretKey") {
			t.Errorf("panic should mention SecretKey, got %v", recovered)
		}
	}()
	settings.Configure(func(s *settings.Settings) { s.Debug = false })
}

func TestEnvHelpers(t *testing.T) {
	t.Setenv("COYOTE_STR", "value")
	t.Setenv("COYOTE_BOOL", "yes")
	t.Setenv("COYOTE_INT", "42")
	t.Setenv("COYOTE_DUR", "90s")
	t.Setenv("COYOTE_LIST", " a , b ,, c ")
	t.Setenv("COYOTE_EMPTY", "")

	if got := settings.Env("COYOTE_STR", "fallback"); got != "value" {
		t.Errorf("settings.Env = %q", got)
	}
	if got := settings.Env("COYOTE_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("empty env should fall back, got %q", got)
	}
	if got := settings.Env("COYOTE_MISSING", "fallback"); got != "fallback" {
		t.Errorf("settings.Env = %q", got)
	}
	if !settings.EnvBool("COYOTE_BOOL", false) {
		t.Error("settings.EnvBool should read yes as true")
	}
	if !settings.EnvBool("COYOTE_MISSING", true) {
		t.Error("settings.EnvBool should fall back")
	}
	if got := settings.EnvInt("COYOTE_INT", 1); got != 42 {
		t.Errorf("settings.EnvInt = %d", got)
	}
	if got := settings.EnvInt("COYOTE_STR", 7); got != 7 {
		t.Errorf("unparsable settings.EnvInt should fall back, got %d", got)
	}
	if got := settings.EnvDuration("COYOTE_DUR", time.Second); got != 90*time.Second {
		t.Errorf("settings.EnvDuration = %v", got)
	}
	if got := settings.EnvList("COYOTE_LIST", nil); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("settings.EnvList = %#v", got)
	}
	if got := settings.EnvList("COYOTE_MISSING", []string{"d"}); len(got) != 1 || got[0] != "d" {
		t.Errorf("settings.EnvList fallback = %#v", got)
	}
}

func TestGenerateSecretKeyIsLongAndUnique(t *testing.T) {
	a, err := settings.GenerateSecretKey()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := settings.GenerateSecretKey()
	if len(a) < 32 {
		t.Errorf("generated key is only %d characters", len(a))
	}
	if a == b {
		t.Error("generated keys should be unique")
	}
}

func panicText(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
