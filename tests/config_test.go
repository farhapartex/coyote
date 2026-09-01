package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/lib/dotenv"
)

func writeFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDotEnvParse(t *testing.T) {
	values, err := dotenv.Parse(strings.NewReader(`
# a comment
export EXPORTED=yes

PLAIN=value
SPACED  =  trimmed
QUOTED="double quoted"
SINGLE='single quoted'
ESCAPED="line\nbreak"
LITERAL='no\nescape'
INLINE=value # trailing comment
HASH_INSIDE="value # kept"
EMPTY=
EQUALS=a=b=c
DOTTED.KEY=allowed
`))
	if err != nil {
		t.Fatal(err)
	}

	for key, want := range map[string]string{
		"EXPORTED":    "yes",
		"PLAIN":       "value",
		"SPACED":      "trimmed",
		"QUOTED":      "double quoted",
		"SINGLE":      "single quoted",
		"ESCAPED":     "line\nbreak",
		"LITERAL":     `no\nescape`,
		"INLINE":      "value",
		"HASH_INSIDE": "value # kept",
		"EMPTY":       "",
		"EQUALS":      "a=b=c",
		"DOTTED.KEY":  "allowed",
	} {
		if got := values[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if _, present := values["#"]; present {
		t.Error("comments should not become variables")
	}
}

func TestDotEnvParseReportsBadLines(t *testing.T) {
	for _, bad := range []string{"NOEQUALS\n", "1INVALID=x\n", `UNTERMINATED="oops` + "\n"} {
		if _, err := dotenv.Parse(strings.NewReader(bad)); err == nil {
			t.Errorf("expected an error for %q", bad)
		} else if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("error should name the line, got %v", err)
		}
	}
}

func TestLoadDotEnvNeverOverridesRealEnvironment(t *testing.T) {
	t.Setenv("COYOTE_FROM_ENV", "real")
	path := writeFile(t, ".env", "COYOTE_FROM_ENV=file\nCOYOTE_FROM_FILE=file\n")

	if err := settings.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Unsetenv("COYOTE_FROM_FILE") })

	if got := os.Getenv("COYOTE_FROM_ENV"); got != "real" {
		t.Errorf("a real environment variable must win, got %q", got)
	}
	if got := os.Getenv("COYOTE_FROM_FILE"); got != "file" {
		t.Errorf("a file-only value should be applied, got %q", got)
	}
}

func TestLoadDotEnvIsOptionalAndOrdered(t *testing.T) {
	if err := settings.LoadDotEnv(filepath.Join(t.TempDir(), "absent.env")); err != nil {
		t.Errorf("a missing .env should not be an error: %v", err)
	}

	first := writeFile(t, "first.env", "COYOTE_ORDER=first\n")
	second := writeFile(t, "second.env", "COYOTE_ORDER=second\n")
	t.Cleanup(func() { os.Unsetenv("COYOTE_ORDER") })

	if err := settings.LoadDotEnv(first, second); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("COYOTE_ORDER"); got != "first" {
		t.Errorf("the first file should win, got %q", got)
	}
}

func TestRequiredDotEnvReportsMissingFile(t *testing.T) {
	source := settings.RequiredDotEnv(filepath.Join(t.TempDir(), "absent.env"))
	if _, err := source.Values(); err == nil {
		t.Error("a required file must fail when missing")
	}
}

func TestMapSourceFeedsEnvHelpers(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("COYOTE_MAP_PORT")
		os.Unsetenv("COYOTE_MAP_DEBUG")
	})
	if err := settings.Load(settings.Map{
		"COYOTE_MAP_PORT":  "9100",
		"COYOTE_MAP_DEBUG": "true",
	}); err != nil {
		t.Fatal(err)
	}
	if got := settings.EnvInt("COYOTE_MAP_PORT", 0); got != 9100 {
		t.Errorf("EnvInt = %d", got)
	}
	if !settings.EnvBool("COYOTE_MAP_DEBUG", false) {
		t.Error("EnvBool should read the loaded value")
	}
}

func TestDotEnvDrivesSettings(t *testing.T) {
	path := writeFile(t, ".env", strings.Join([]string{
		`SECRET_KEY="` + strings.Repeat("k", 48) + `"`,
		"PORT=9200",
		"ALLOWED_HOSTS=example.com, www.example.com",
	}, "\n"))
	t.Cleanup(func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("PORT")
		os.Unsetenv("ALLOWED_HOSTS")
	})
	if err := settings.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}

	resolved, err := settings.New(func(s *settings.Settings) {
		s.Environment = settings.Production
		s.SecretKey = settings.Env("SECRET_KEY", "")
		s.Server.Port = settings.EnvInt("PORT", 8000)
		s.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Server.Port != 9200 {
		t.Errorf("Port = %d", resolved.Server.Port)
	}
	if len(resolved.AllowedHosts) != 2 || resolved.AllowedHosts[1] != "www.example.com" {
		t.Errorf("AllowedHosts = %#v", resolved.AllowedHosts)
	}
	if len(resolved.SecretKey) != 48 {
		t.Errorf("SecretKey length = %d", len(resolved.SecretKey))
	}
}

func TestDevelopmentPreset(t *testing.T) {
	resolved, err := settings.New(settings.Preset("dev"))
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.IsDevelopment() || resolved.Environment != settings.Development {
		t.Errorf("Environment = %q", resolved.Environment)
	}
	if !resolved.Debug {
		t.Error("development should enable Debug")
	}
	if resolved.Logging.Level != "debug" || resolved.Logging.Format != "text" {
		t.Errorf("logging = %+v", resolved.Logging)
	}
	if resolved.Sessions.Secure {
		t.Error("development should not require secure cookies")
	}
	if len(resolved.AllowedHosts) == 0 {
		t.Error("development should default AllowedHosts to localhost")
	}
	if !resolved.AutoReloadTemplates() {
		t.Error("templates should reload in development")
	}
}

func TestProductionPreset(t *testing.T) {
	resolved, err := settings.New(
		settings.Preset("production"),
		func(s *settings.Settings) {
			s.SecretKey = strings.Repeat("k", 48)
			s.AllowedHosts = []string{"example.com"}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.IsProduction() || !resolved.IsDeployed() {
		t.Errorf("Environment = %q", resolved.Environment)
	}
	if resolved.Debug {
		t.Error("production must not enable Debug")
	}
	if resolved.Logging.Format != "json" || resolved.Logging.Level != "info" {
		t.Errorf("logging = %+v", resolved.Logging)
	}
	if !resolved.Sessions.Secure {
		t.Error("production should require secure cookies")
	}
	if resolved.Server.ReadTimeout != 15*time.Second || resolved.Server.WriteTimeout != 30*time.Second {
		t.Errorf("timeouts = %v / %v", resolved.Server.ReadTimeout, resolved.Server.WriteTimeout)
	}
	if resolved.AutoReloadTemplates() {
		t.Error("templates must be cached in production")
	}
}

func TestStagingPresetMatchesProductionHardening(t *testing.T) {
	resolved, err := settings.New(
		settings.Preset("stage"),
		func(s *settings.Settings) {
			s.SecretKey = strings.Repeat("k", 48)
			s.AllowedHosts = []string{"staging.example.com"}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.IsStaging() || !resolved.IsDeployed() {
		t.Errorf("Environment = %q", resolved.Environment)
	}
	if resolved.Debug || !resolved.Sessions.Secure {
		t.Error("staging should be hardened like production")
	}
}

func TestPresetIsOverridableAfterwards(t *testing.T) {
	resolved, err := settings.New(
		settings.Preset("production"),
		func(s *settings.Settings) {
			s.SecretKey = strings.Repeat("k", 48)
			s.AllowedHosts = []string{"example.com"}
			s.Logging.Format = "text"
			s.Server.ReadTimeout = time.Minute
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Logging.Format != "text" {
		t.Error("a later mutator should win over the preset")
	}
	if resolved.Server.ReadTimeout != time.Minute {
		t.Errorf("ReadTimeout = %v", resolved.Server.ReadTimeout)
	}
}

func TestUnknownProfileIsRejected(t *testing.T) {
	_, err := settings.New(
		settings.Preset("banana"),
		func(s *settings.Settings) {
			s.SecretKey = strings.Repeat("k", 48)
			s.AllowedHosts = []string{"example.com"}
		},
	)
	if err == nil {
		t.Fatal("an unknown environment should be rejected")
	}
	mustContain(t, problemsOf(t, err), "not recognised")
}

func TestDebugIsRejectedInDeployedEnvironments(t *testing.T) {
	for _, environment := range []settings.Profile{settings.Staging, settings.Production} {
		_, err := settings.New(func(s *settings.Settings) {
			s.Environment = environment
			s.Debug = true
			s.SecretKey = strings.Repeat("k", 48)
			s.AllowedHosts = []string{"example.com"}
		})
		if err == nil {
			t.Fatalf("%s with Debug should be rejected", environment)
		}
		mustContain(t, problemsOf(t, err), "Debug must be off")
	}
}

func TestProfileAliases(t *testing.T) {
	for name, want := range map[string]settings.Profile{
		"dev": settings.Development, "development": settings.Development, "local": settings.Development,
		"stage": settings.Staging, "staging": settings.Staging,
		"prod": settings.Production, "production": settings.Production, "LIVE": settings.Production,
	} {
		if got, ok := settings.ParseProfile(name); !ok || got != want {
			t.Errorf("ParseProfile(%q) = %q/%t, want %q", name, got, ok, want)
		}
	}
	if _, ok := settings.ParseProfile("banana"); ok {
		t.Error("an unknown name should not resolve")
	}
}

func TestDefaultEnvironmentIsDevelopment(t *testing.T) {
	resolved, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Environment != settings.Development {
		t.Errorf("Environment = %q, want development by default", resolved.Environment)
	}
	if resolved.Debug {
		t.Error("the default must not enable Debug on its own")
	}
}

func TestSQLitePoolIsPinnedToOneConnection(t *testing.T) {
	resolved, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatal(err)
	}
	db := resolved.Database()
	if db.MaxOpenConns != 1 || db.MaxIdleConns != 1 {
		t.Errorf("pool = %d open, %d idle; SQLite wants one so it never meets SQLITE_BUSY",
			db.MaxOpenConns, db.MaxIdleConns)
	}
}

func TestAServerEnginePoolIsBoundedAndRecycled(t *testing.T) {
	resolved, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Databases = []settings.Database{{Engine: settings.Postgres, Name: "shop"}}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	db := resolved.Database()
	if db.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %d, want 25 rather than unlimited", db.MaxOpenConns)
	}
	if db.MaxIdleConns != db.MaxOpenConns {
		t.Errorf("MaxIdleConns = %d, want it to match MaxOpenConns so connections are not churned",
			db.MaxIdleConns)
	}
	if db.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 30m so a proxy cannot hand back a dead connection",
			db.ConnMaxLifetime)
	}
}

func TestAnExplicitPoolSettingIsLeftAlone(t *testing.T) {
	resolved, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Databases = []settings.Database{{
			Engine: settings.Postgres, Name: "shop", MaxOpenConns: 4, ConnMaxLifetime: time.Minute,
		}}
	})...)
	if err != nil {
		t.Fatal(err)
	}
	db := resolved.Database()
	if db.MaxOpenConns != 4 || db.ConnMaxLifetime != time.Minute {
		t.Errorf("pool = %d open, %v lifetime; what the project set should survive",
			db.MaxOpenConns, db.ConnMaxLifetime)
	}
}
