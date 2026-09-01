package tests

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

type greetCommand struct{ ran *bool }

func (greetCommand) Name() string { return "greet" }

func (greetCommand) Summary() string { return "say hello" }

func (c greetCommand) Run(ctx cli.Context) error {
	*c.ran = true
	ctx.Out.Write([]byte("hello " + strings.Join(cli.Args(), " ") + "\n"))
	return nil
}

func TestApplicationsCanRegisterCommands(t *testing.T) {
	ran := false
	cli.Register(greetCommand{ran: &ran})

	registry := cli.Default()
	if _, found := registry.Lookup("greet"); !found {
		t.Fatalf("a registered command should be dispatchable, known: %v", registry.Names())
	}

	out := &bytes.Buffer{}
	a := newTestApp(t)
	if err := registry.Run(cli.Context{App: a, Out: out, In: strings.NewReader("")}, "greet"); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("the command did not run")
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("output = %q", out.String())
	}
}

func TestAnUnknownCommandStillErrors(t *testing.T) {
	err := cli.Default().Run(cli.Context{App: newTestApp(t), Out: &bytes.Buffer{}}, "no-such-command")
	if !errors.Is(err, cli.ErrUnknownCommand) {
		t.Errorf("error = %v, want ErrUnknownCommand", err)
	}
}

func shellApp(t *testing.T) *app.App {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) { s.Logging.Level = "error" }))
	a.RegisterModel(model.Of(Widget2{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Create(&Widget2{ID: "w1", Name: "First", Stock: 3}).Error; err != nil {
		t.Fatal(err)
	}
	return a
}

func runShell(t *testing.T, a *app.App, script string) string {
	t.Helper()
	out := &bytes.Buffer{}
	if err := (cli.Shell{}).Run(cli.Context{App: a, Out: out, In: strings.NewReader(script)}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestShellListsAndDescribesModels(t *testing.T) {
	a := shellApp(t)

	body := runShell(t, a, ".models\n.quit\n")
	if !strings.Contains(body, "widget2") {
		t.Errorf(".models should list the registered tables:\n%s", body)
	}

	body = runShell(t, a, ".describe widget2\n.quit\n")
	for _, want := range []string{"id", "name", "stock", "primary key"} {
		if !strings.Contains(body, want) {
			t.Errorf(".describe should mention %q:\n%s", want, body)
		}
	}
}

func TestShellCountsListsAndGets(t *testing.T) {
	a := shellApp(t)

	if body := runShell(t, a, "count widget2\n.quit\n"); !strings.Contains(body, "1") {
		t.Errorf("count = %s", body)
	}
	if body := runShell(t, a, "list widget2\n.quit\n"); !strings.Contains(body, "name=First") {
		t.Errorf("list should print rows:\n%s", body)
	}
	if body := runShell(t, a, "get widget2 w1\n.quit\n"); !strings.Contains(body, "name=First") {
		t.Errorf("get should find the row:\n%s", body)
	}
}

func TestShellReportsMistakesWithoutQuitting(t *testing.T) {
	a := shellApp(t)

	body := runShell(t, a, "count nope\nlist\nfly\ncount widget2\n.quit\n")
	if !strings.Contains(body, `no model called "nope"`) {
		t.Errorf("an unknown model should be reported:\n%s", body)
	}
	if !strings.Contains(body, "usage: list") {
		t.Errorf("a missing argument should show usage:\n%s", body)
	}
	if !strings.Contains(body, `unknown command "fly"`) {
		t.Errorf("an unknown command should be reported:\n%s", body)
	}
	if !strings.Contains(body, "> 1") {
		t.Errorf("the shell should keep going after mistakes:\n%s", body)
	}
}

func TestShellHelpListsTheCommands(t *testing.T) {
	body := runShell(t, shellApp(t), ".help\n.quit\n")
	for _, want := range []string{".models", ".describe", "count", "list", "get", ".quit"} {
		if !strings.Contains(body, want) {
			t.Errorf(".help should mention %q:\n%s", want, body)
		}
	}
}

func TestDBShellNamesTheClientItNeeds(t *testing.T) {
	a := newTestApp(t)
	out := &bytes.Buffer{}

	err := (cli.DBShell{}).Run(cli.Context{App: a, Out: out, In: strings.NewReader("")})
	if err == nil {
		return
	}
	if !strings.Contains(err.Error(), "sqlite3") {
		t.Errorf("without the client installed the error should name it, got %v", err)
	}
}

func TestCreateSuperadminStillTakesAPipedPassword(t *testing.T) {
	a := newTestApp(t)
	syncSchema(t, a)

	out := &bytes.Buffer{}
	input := strings.NewReader("piped\npiped@example.com\nunrelated-and-long\n")
	if err := (cli.CreateSuperadmin{}).Run(cli.Context{App: a, Out: out, In: input}); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Auth.Authenticate(t.Context(), "piped", "unrelated-and-long"); err != nil {
		t.Errorf("a piped password should still work when stdin is not a terminal: %v", err)
	}
}
