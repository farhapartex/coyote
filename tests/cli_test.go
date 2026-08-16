package tests

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

type fakeApp struct {
	served   bool
	cfg      settings.Settings
	models   []model.Model
	handle   *gorm.DB
	dbErr    error
	serveErr error
	auth     *auth.Service
}

func (f *fakeApp) Serve() error {
	f.served = true
	return f.serveErr
}

func (f *fakeApp) Config() settings.Settings { return f.cfg }

func (f *fakeApp) Models() []model.Model { return f.models }

func (f *fakeApp) DB() (*gorm.DB, error) { return f.handle, f.dbErr }

func (f *fakeApp) Log() *slog.Logger { return slog.Default() }

func (f *fakeApp) AuthService() *auth.Service {
	if f.auth == nil {
		service, _ := newTestAuth()
		f.auth = service
	}
	return f.auth
}

func listenOnFreePort(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener.Addr().String(), func() { listener.Close() }
}

func settingsForAddr(t *testing.T, addr string) settings.Settings {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	return devSettings(t, func(s *settings.Settings) {
		s.Server.Host = host
		s.Server.Port = atoi(t, port)
	})
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("not a port: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func TestStartCommandServes(t *testing.T) {
	app := &fakeApp{cfg: devSettings(t)}
	out := &bytes.Buffer{}
	if err := cli.Default().Run(cli.Context{App: app, Out: out}, cli.NameStart); err != nil {
		t.Fatal(err)
	}
	if !app.served {
		t.Error("start should call Serve")
	}
}

func TestEmptyCommandDefaultsToStart(t *testing.T) {
	app := &fakeApp{cfg: devSettings(t)}
	if err := cli.Default().Run(cli.Context{App: app, Out: &bytes.Buffer{}}, ""); err != nil {
		t.Fatal(err)
	}
	if !app.served {
		t.Error("an empty command should default to start")
	}
}

func TestUnknownCommandIsRejected(t *testing.T) {
	app := &fakeApp{cfg: devSettings(t)}
	err := cli.Default().Run(cli.Context{App: app, Out: &bytes.Buffer{}}, "danceoff")
	if !errors.Is(err, cli.ErrUnknownCommand) {
		t.Fatalf("got %v, want ErrUnknownCommand", err)
	}
	if !strings.Contains(err.Error(), "migrate") {
		t.Errorf("the error should list known commands, got %v", err)
	}
	if app.served {
		t.Error("an unknown command must not serve")
	}
}

func TestMigrateRefusesWhenServerIsDown(t *testing.T) {
	addr, stop := listenOnFreePort(t)
	cfg := settingsForAddr(t, addr)
	stop()

	app := &fakeApp{cfg: cfg, models: []model.Model{model.Of(auth.User{})}}
	out := &bytes.Buffer{}
	err := cli.Default().Run(cli.Context{App: app, Out: out}, cli.NameMigrate)
	if !errors.Is(err, cli.ErrServerNotRunning) {
		t.Fatalf("got %v, want ErrServerNotRunning", err)
	}
	if !strings.Contains(err.Error(), "coyote start") {
		t.Errorf("the error should tell the user how to start, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be migrated, got output %q", out.String())
	}
}

func TestMigrateWithNoDeclaredMigrations(t *testing.T) {
	addr, stop := listenOnFreePort(t)
	defer stop()

	cfg := settingsForAddr(t, addr)
	handle := newTestDB(t)
	app := &fakeApp{cfg: cfg, models: []model.Model{model.Of(auth.User{})}, handle: handle}

	out := &bytes.Buffer{}
	if err := cli.Default().Run(cli.Context{App: app, Out: out}, cli.NameMigrate); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{"server", "running on " + addr, "no migrations declared"} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q:\n%s", want, body)
		}
	}
	if !handle.Migrator().HasTable(migrate.LedgerTable) {
		t.Errorf("migrate should create the %s ledger", migrate.LedgerTable)
	}
}

func TestServerRunningProbe(t *testing.T) {
	addr, stop := listenOnFreePort(t)
	if !cli.ServerRunning(addr) {
		t.Error("probe should find a live listener")
	}
	stop()
	if cli.ServerRunning(addr) {
		t.Error("probe should not find a closed listener")
	}
	if err := cli.RequireServer(addr); !errors.Is(err, cli.ErrServerNotRunning) {
		t.Errorf("got %v, want ErrServerNotRunning", err)
	}
}
