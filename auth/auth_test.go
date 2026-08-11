package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/farhapartex/coyote/session"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse battery", hash) {
		t.Error("correct password should verify")
	}
	if VerifyPassword("wrong password", hash) {
		t.Error("wrong password should not verify")
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("same password")
	b, _ := HashPassword("same password")
	if a == b {
		t.Error("two hashes of the same password should differ")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "plaintext", "md5$1$x$y", "pbkdf2_sha256$notanint$x$y"} {
		if VerifyPassword("anything", bad) {
			t.Errorf("malformed hash %q should not verify", bad)
		}
	}
}

func newTestService() *Service {
	manager := session.NewManager(session.Options{
		Store:    session.NewMemoryStore(0),
		Lifetime: time.Hour,
	})
	return NewService(NewMemoryStore(), manager, Options{
		Hasher:            Hasher{Iterations: 1000},
		MinPasswordLength: 8,
	})
}

func TestCreateAndAuthenticate(t *testing.T) {
	s := newTestService()
	created, err := s.CreateUser(NewUser{
		Username:  "jane",
		Email:     "jane@example.com",
		FirstName: "Jane",
		LastName:  "Doe",
		Password:  "supersecret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.FullName() != "Jane Doe" || created.DisplayName() != "Jane Doe" {
		t.Errorf("name handling: %q / %q", created.FullName(), created.DisplayName())
	}
	if created.Initials() != "JD" {
		t.Errorf("Initials = %q, want JD", created.Initials())
	}
	if !created.IsActive {
		t.Error("new users should be active")
	}
	if created.IsSuperadmin {
		t.Error("new users should not be superadmin by default")
	}
	if created.Password == "supersecret" {
		t.Fatal("password was stored in plain text")
	}
	if !created.HasUsablePassword() {
		t.Error("stored password should be a recognised hash")
	}
	if created.HasLoggedIn() {
		t.Error("a new user has never logged in")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("timestamps should be set on create")
	}

	user, err := s.Authenticate("JANE", "supersecret")
	if err != nil {
		t.Fatalf("case-insensitive login failed: %v", err)
	}
	if user.ID != created.ID {
		t.Error("Authenticate returned a different user")
	}
	if _, err := s.Authenticate("jane", "nope"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Authenticate("ghost", "supersecret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user should give ErrInvalidCredentials, got %v", err)
	}
}

func TestCreateSuperadmin(t *testing.T) {
	s := newTestService()
	user, err := s.CreateSuperadmin("root", "root@example.com", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	if !user.IsSuperadmin || !user.IsActive {
		t.Errorf("unexpected superadmin: %+v", user)
	}
	if user.Initials() != "R" {
		t.Errorf("Initials should fall back to the username, got %q", user.Initials())
	}
	if user.DisplayName() != "root" {
		t.Errorf("DisplayName should fall back to the username, got %q", user.DisplayName())
	}
}

func TestEmailMustBeUniqueAndWellFormed(t *testing.T) {
	s := newTestService()
	if _, err := s.CreateUser(NewUser{Username: "jane", Email: "jane@example.com", Password: "supersecret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(NewUser{Username: "other", Email: "JANE@example.com", Password: "supersecret"}); !errors.Is(err, ErrEmailExists) {
		t.Errorf("got %v, want ErrEmailExists", err)
	}
	if _, err := s.CreateUser(NewUser{Username: "third", Email: "not-an-email", Password: "supersecret"}); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("got %v, want ErrInvalidEmail", err)
	}
	if _, err := s.CreateUser(NewUser{Username: "fourth", Password: "supersecret"}); err != nil {
		t.Errorf("a blank email should be allowed: %v", err)
	}
	if _, err := s.CreateUser(NewUser{Username: "fifth", Password: "supersecret"}); err != nil {
		t.Errorf("multiple blank emails should be allowed: %v", err)
	}
}

func TestLoginStampsLastLoginAt(t *testing.T) {
	s := newTestService()
	created, err := s.CreateSuperadmin("root", "", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	if created.HasLoggedIn() {
		t.Fatal("should not have a login stamp yet")
	}

	rec := httptest.NewRecorder()
	s.sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.Authenticate("root", "supersecret")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Login(r, user); err != nil {
			t.Fatal(err)
		}
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	stored, err := s.Users().ByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.HasLoggedIn() {
		t.Error("LastLoginAt was not stamped on login")
	}
	if !stored.UpdatedAt.After(created.UpdatedAt) && !stored.UpdatedAt.Equal(created.UpdatedAt) {
		t.Error("UpdatedAt should advance on update")
	}
}

func TestInactiveUserCannotAuthenticate(t *testing.T) {
	s := newTestService()
	user, _ := s.CreateUser(NewUser{Username: "jane", Password: "supersecret"})
	user.IsActive = false
	if err := s.Users().Update(user); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate("jane", "supersecret"); !errors.Is(err, ErrInactiveAccount) {
		t.Errorf("got %v, want ErrInactiveAccount", err)
	}
}

func TestDuplicateAndInvalidUsernames(t *testing.T) {
	s := newTestService()
	if _, err := s.CreateUser(NewUser{Username: "jane", Password: "supersecret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(NewUser{Username: "Jane", Password: "supersecret"}); !errors.Is(err, ErrUserExists) {
		t.Errorf("got %v, want ErrUserExists", err)
	}
	if _, err := s.CreateUser(NewUser{Username: "ab", Password: "supersecret"}); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("got %v, want ErrInvalidUser", err)
	}
	if _, err := s.CreateUser(NewUser{Username: "valid", Password: "short"}); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("got %v, want ErrPasswordTooShort", err)
	}
}

func TestCannotRemoveLastSuperadmin(t *testing.T) {
	s := newTestService()
	root, _ := s.CreateSuperadmin("root", "", "supersecret")
	if err := s.Users().Delete(root.ID); !errors.Is(err, ErrLastSuperadmin) {
		t.Errorf("got %v, want ErrLastSuperadmin", err)
	}

	demoted := root.Clone()
	demoted.IsSuperadmin = false
	if err := s.Users().Update(demoted); !errors.Is(err, ErrLastSuperadmin) {
		t.Errorf("demoting the last superadmin: got %v, want ErrLastSuperadmin", err)
	}
	deactivated := root.Clone()
	deactivated.IsActive = false
	if err := s.Users().Update(deactivated); !errors.Is(err, ErrLastSuperadmin) {
		t.Errorf("disabling the last superadmin: got %v, want ErrLastSuperadmin", err)
	}

	second, _ := s.CreateSuperadmin("root2", "", "supersecret")
	if err := s.Users().Delete(root.ID); err != nil {
		t.Errorf("deleting one of two superadmins should succeed: %v", err)
	}
	if err := s.Users().Delete(second.ID); !errors.Is(err, ErrLastSuperadmin) {
		t.Errorf("got %v, want ErrLastSuperadmin", err)
	}
}

func TestInactiveSuperadminIsNotCountedAsTheLastOne(t *testing.T) {
	s := newTestService()
	active, _ := s.CreateSuperadmin("root", "", "supersecret")
	spare, _ := s.CreateSuperadmin("spare", "", "supersecret")

	disabled := spare.Clone()
	disabled.IsActive = false
	if err := s.Users().Update(disabled); err != nil {
		t.Fatalf("disabling one of two superadmins should succeed: %v", err)
	}

	demoted := disabled.Clone()
	demoted.IsSuperadmin = false
	if err := s.Users().Update(demoted); err != nil {
		t.Errorf("demoting an already inactive superadmin should succeed: %v", err)
	}
	if err := s.Users().Delete(spare.ID); err != nil {
		t.Errorf("deleting an inactive superadmin should succeed: %v", err)
	}
	if _, err := s.Users().ByID(active.ID); err != nil {
		t.Error("the active superadmin should still exist")
	}
}

func TestLoginAndGuards(t *testing.T) {
	s := newTestService()
	manager := s.sessions
	if _, err := s.CreateUser(NewUser{Username: "jane", Password: "supersecret"}); err != nil {
		t.Fatal(err)
	}

	protected := manager.Middleware(s.Middleware(s.RequireSuperadmin("/login")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))))

	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("anonymous got %d, want 303", rec.Code)
	}

	login := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.Authenticate("jane", "supersecret")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Login(r, user); err != nil {
			t.Fatal(err)
		}
	}))
	rec = httptest.NewRecorder()
	login.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-superadmin got %d, want 403", rec.Code)
	}

	loginOnly := manager.Middleware(s.Middleware(s.RequireLogin("/login")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u := UserFrom(r.Context()); u == nil || u.Username != "jane" {
				t.Error("expected jane in context")
			}
		}))))
	req = httptest.NewRequest(http.MethodGet, "/me/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	loginOnly.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated got %d, want 200", rec.Code)
	}
}
