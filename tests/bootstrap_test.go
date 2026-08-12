package tests

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/repo"
	"github.com/farhapartex/coyote/core/settings"
)

func captureLogs(t *testing.T) (*bytes.Buffer, *slog.Logger) {
	t.Helper()
	buffer := &bytes.Buffer{}
	handler := slog.NewTextHandler(buffer, &slog.HandlerOptions{Level: slog.LevelInfo})
	return buffer, slog.New(handler)
}

func TestUsersLiveInTheDatabase(t *testing.T) {
	a := newTestApp(t)

	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "supersecret"); err != nil {
		t.Fatal(err)
	}

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	var rows int64
	if err := handle.Table("users").Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("users table holds %d rows, want 1", rows)
	}

	second := app.NewFrom(a.Settings)
	found, err := second.Auth.Users().ByUsername("root")
	if err != nil {
		t.Fatalf("a second process should see the user: %v", err)
	}
	if !found.IsSuperadmin {
		t.Error("the persisted user should still be a superadmin")
	}
	if _, err := second.Auth.Authenticate("root", "supersecret"); err != nil {
		t.Errorf("the persisted password should verify: %v", err)
	}
}

func TestDatabaseUserStoreBehaviour(t *testing.T) {
	a := newTestApp(t)
	store := a.Auth.Users()

	created, err := a.Auth.CreateUser(auth.NewUser{
		Username: "jane", Email: "Jane@Example.com", FirstName: "Jane", Password: "supersecret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("the store must assign an id")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("timestamps should be set on create")
	}

	byName, err := store.ByUsername("JANE")
	if err != nil || byName.ID != created.ID {
		t.Errorf("case-insensitive username lookup failed: %v", err)
	}
	byEmail, err := store.ByEmail("jane@example.com")
	if err != nil || byEmail.ID != created.ID {
		t.Errorf("case-insensitive email lookup failed: %v", err)
	}
	if _, err := store.ByID("missing"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("got %v, want ErrUserNotFound", err)
	}

	if _, err := a.Auth.CreateUser(auth.NewUser{Username: "Jane", Password: "supersecret"}); !errors.Is(err, auth.ErrUserExists) {
		t.Errorf("duplicate username: got %v, want ErrUserExists", err)
	}
	if _, err := a.Auth.CreateUser(auth.NewUser{Username: "other", Email: "JANE@example.com", Password: "supersecret"}); !errors.Is(err, auth.ErrEmailExists) {
		t.Errorf("duplicate email: got %v, want ErrEmailExists", err)
	}
	if _, err := a.Auth.CreateUser(auth.NewUser{Username: "blank1", Password: "supersecret"}); err != nil {
		t.Errorf("a blank email should be allowed: %v", err)
	}
	if _, err := a.Auth.CreateUser(auth.NewUser{Username: "blank2", Password: "supersecret"}); err != nil {
		t.Errorf("multiple blank emails should be allowed: %v", err)
	}

	created.FirstName = "Janet"
	if err := store.Update(created); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.ByID(created.ID)
	if reloaded.FirstName != "Janet" {
		t.Errorf("update did not persist: %+v", reloaded)
	}
	if !reloaded.CreatedAt.Equal(created.CreatedAt) {
		t.Error("update must not move CreatedAt")
	}

	if store.Count() != 3 {
		t.Errorf("Count = %d, want 3", store.Count())
	}
	if len(store.All()) != 3 {
		t.Errorf("All returned %d users", len(store.All()))
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ByID(created.ID); !errors.Is(err, auth.ErrUserNotFound) {
		t.Error("user should be deleted")
	}
	if err := store.Delete("missing"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("got %v, want ErrUserNotFound", err)
	}
}

func TestLastSuperadminGuardAppliesToEveryStore(t *testing.T) {
	for name, store := range map[string]auth.Store{
		"memory":   auth.NewMemoryStore(),
		"database": nil,
	} {
		t.Run(name, func(t *testing.T) {
			target := store
			if target == nil {
				handle := newTestDB(t)
				if err := migrate.Sync(handle, []model.Model{model.Of(auth.User{})}); err != nil {
					t.Fatal(err)
				}
				target = repo.Users(handle)
			}
			service := auth.NewService(target, newTestManager(), auth.Options{
				Hasher: auth.Hasher{Iterations: 1000}, MinPasswordLength: 8,
			})

			root, err := service.CreateSuperadmin("root", "", "supersecret")
			if err != nil {
				t.Fatal(err)
			}
			if err := service.Users().Delete(root.ID); !errors.Is(err, auth.ErrLastSuperadmin) {
				t.Errorf("delete: got %v, want ErrLastSuperadmin", err)
			}
			demoted := root.Clone()
			demoted.IsSuperadmin = false
			if err := service.Users().Update(demoted); !errors.Is(err, auth.ErrLastSuperadmin) {
				t.Errorf("demote: got %v, want ErrLastSuperadmin", err)
			}

			if _, err := service.CreateSuperadmin("spare", "", "supersecret"); err != nil {
				t.Fatal(err)
			}
			if err := service.Users().Delete(root.ID); err != nil {
				t.Errorf("deleting one of two superadmins should succeed: %v", err)
			}
		})
	}
}

func TestStartWarnsWhenNothingIsMigrated(t *testing.T) {
	buffer, logger := captureLogs(t)
	resolved := devSettings(t, func(s *settings.Settings) { s.Logging.Logger = logger })
	a := app.NewFrom(resolved)

	cli.ReportMigrationState(a)

	logged := buffer.String()
	if !strings.Contains(logged, "no migrations") {
		t.Errorf("expected a migration warning, got:\n%s", logged)
	}
	if !strings.Contains(logged, "coyote makemigrations") {
		t.Errorf("the warning should tell the user what to run, got:\n%s", logged)
	}
}

func TestStartWarnsWhenMigratedButNoUsers(t *testing.T) {
	buffer, logger := captureLogs(t)
	resolved := devSettings(t, func(s *settings.Settings) { s.Logging.Logger = logger })
	a := app.NewFrom(resolved)
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	runner := migrate.NewRunner(handle, settings.SQLite, nil)
	if err := runner.Prepare(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply(t.Context(), migrate.Migration{ID: "0001_baseline", Up: []migrate.Op{
		migrate.RunSQL{Any: "SELECT 1"},
	}}); err != nil {
		t.Fatal(err)
	}

	buffer.Reset()
	cli.ReportMigrationState(a)

	logged := buffer.String()
	if !strings.Contains(logged, "no users yet") {
		t.Errorf("expected a no-users warning, got:\n%s", logged)
	}
	if !strings.Contains(logged, "createsuperadmin") {
		t.Errorf("the warning should point at createsuperadmin, got:\n%s", logged)
	}
}

func TestStartIsQuietWhenEverythingIsReady(t *testing.T) {
	buffer, logger := captureLogs(t)
	resolved := devSettings(t, func(s *settings.Settings) { s.Logging.Logger = logger })
	a := app.NewFrom(resolved)
	syncSchema(t, a)

	handle, _ := a.DB()
	runner := migrate.NewRunner(handle, settings.SQLite, nil)
	if err := runner.Prepare(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply(t.Context(), migrate.Migration{ID: "0001_noop", Up: []migrate.Op{
		migrate.RunSQL{Any: "SELECT 1"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Auth.CreateSuperadmin("root", "", "supersecret"); err != nil {
		t.Fatal(err)
	}

	buffer.Reset()
	cli.ReportMigrationState(a)

	logged := buffer.String()
	if strings.Contains(logged, "level=WARN") {
		t.Errorf("a migrated database with a user should not warn:\n%s", logged)
	}
	if !strings.Contains(logged, "migrations up to date") {
		t.Errorf("expected an up-to-date line, got:\n%s", logged)
	}
}

func TestCreateSuperadminRequiresSchema(t *testing.T) {
	resolved := devSettings(t)
	a := app.NewFrom(resolved)

	out := &bytes.Buffer{}
	command := cli.CreateSuperadmin{Username: "root", Email: "root@example.com", Password: "supersecret"}
	err := command.Run(cli.Context{App: a, Out: out, In: strings.NewReader("")})
	if !errors.Is(err, cli.ErrSchemaMissing) {
		t.Fatalf("got %v, want ErrSchemaMissing", err)
	}
	if !strings.Contains(err.Error(), "coyote migrate") {
		t.Errorf("the error should tell the user to migrate, got %v", err)
	}
}

func TestCreateSuperadminFromFlags(t *testing.T) {
	a := newTestApp(t)

	out := &bytes.Buffer{}
	command := cli.CreateSuperadmin{Username: "root", Email: "root@example.com", Password: "supersecret"}
	if err := command.Run(cli.Context{App: a, Out: out, In: strings.NewReader("")}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "created superadmin root") {
		t.Errorf("unexpected output: %s", out.String())
	}

	created, err := a.Auth.Users().ByUsername("root")
	if err != nil {
		t.Fatal(err)
	}
	if !created.IsSuperadmin || !created.IsActive {
		t.Errorf("unexpected user: %+v", created)
	}
	if created.Password == "supersecret" {
		t.Fatal("the password must be hashed")
	}
	if _, err := a.Auth.Authenticate("root", "supersecret"); err != nil {
		t.Errorf("the new superadmin should be able to sign in: %v", err)
	}
}

func TestCreateSuperadminPromptsWhenFlagsAreMissing(t *testing.T) {
	a := newTestApp(t)

	out := &bytes.Buffer{}
	input := strings.NewReader("prompted\nprompted@example.com\nsupersecret\n")
	if err := (cli.CreateSuperadmin{}).Run(cli.Context{App: a, Out: out, In: input}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Username:", "Email:", "Password:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing prompt %q in:\n%s", want, out.String())
		}
	}
	if _, err := a.Auth.Users().ByUsername("prompted"); err != nil {
		t.Errorf("prompted user was not created: %v", err)
	}
}

func TestCreateSuperadminRejectsWeakPassword(t *testing.T) {
	a := newTestApp(t)
	command := cli.CreateSuperadmin{Username: "root", Password: "short"}
	err := command.Run(cli.Context{App: a, Out: &bytes.Buffer{}, In: strings.NewReader("")})
	if !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("got %v, want ErrPasswordTooShort", err)
	}
	if a.Auth.Users().Count() != 0 {
		t.Error("a rejected password must not create a user")
	}
}
