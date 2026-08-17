package tests

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

type Report struct {
	ID    string `gorm:"primaryKey;size:64"`
	Label string `gorm:"size:100;not null"`
}

func twoDatabases(t *testing.T) *app.App {
	t.Helper()
	dir := t.TempDir()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Migrations.Dir = filepath.Join(dir, "migrations")
		s.Databases = []settings.Database{
			{Alias: "default", Engine: settings.SQLite, Name: filepath.Join(dir, "main.db"), MaxOpenConns: 1},
			{Alias: "reports", Engine: settings.SQLite, Name: filepath.Join(dir, "reports.db"), MaxOpenConns: 1},
		}
	}))
	a.RegisterModel(model.Of(Widget2{}), model.On("reports", Report{}))
	return a
}

func TestModelsCanDeclareTheirDatabase(t *testing.T) {
	a := twoDatabases(t)

	if got := a.Registry().AliasOf(Report{}); got != "reports" {
		t.Errorf("AliasOf(Report) = %q", got)
	}
	if got := a.Registry().AliasOf(Widget2{}); got != "" {
		t.Errorf("an unrouted model should report no alias, got %q", got)
	}
	if got := a.Registry().Aliases(); len(got) != 1 || got[0] != "reports" {
		t.Errorf("Aliases = %v", got)
	}
	if got := len(a.Registry().For("reports")); got != 1 {
		t.Errorf("For(reports) returned %d models", got)
	}
}

func TestConnectionsAreSeparatePerAlias(t *testing.T) {
	a := twoDatabases(t)

	main, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	reports, err := a.DBByAlias("reports")
	if err != nil {
		t.Fatal(err)
	}
	if main == reports {
		t.Fatal("each alias should get its own connection")
	}

	again, err := a.DBByAlias("reports")
	if err != nil {
		t.Fatal(err)
	}
	if again != reports {
		t.Error("connections should be memoised, not reopened per call")
	}

	if _, err := a.DBByAlias("nope"); err == nil {
		t.Error("an unknown alias should be an error")
	}

	routed, err := a.DBFor(Report{})
	if err != nil {
		t.Fatal(err)
	}
	if routed != reports {
		t.Error("DBFor should follow the model's alias")
	}
	unrouted, err := a.DBFor(Widget2{})
	if err != nil {
		t.Fatal(err)
	}
	if unrouted != main {
		t.Error("an unrouted model should use the default connection")
	}
}

func TestWritesLandInTheRoutedDatabase(t *testing.T) {
	a := twoDatabases(t)

	reports, err := a.DBByAlias("reports")
	if err != nil {
		t.Fatal(err)
	}
	if err := reports.AutoMigrate(&Report{}); err != nil {
		t.Fatal(err)
	}

	records, err := a.StoreFor(Report{})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := a.Describe(Report{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := records.Insert(t.Context(), schema, model.Record{"id": "r1", "label": "Weekly"}); err != nil {
		t.Fatal(err)
	}

	total, err := records.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("the routed store counted %d", total)
	}

	main, _ := a.DB()
	if main.Migrator().HasTable("reports") {
		t.Error("the routed table must not appear in the default database")
	}
}

func TestRoutedModelsAreReportedByMigrate(t *testing.T) {
	a := twoDatabases(t)
	syncSchema(t, a)

	out := &bytes.Buffer{}
	command := cli.MakeMigrations{Label: "initial"}
	if err := command.Run(cli.Context{App: a, Out: out, In: strings.NewReader("")}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "routed to another database") {
		t.Errorf("makemigrations should say what it skipped:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "reports") {
		t.Errorf("the alias should be named:\n%s", out.String())
	}
}
