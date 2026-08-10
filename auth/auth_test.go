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
	return NewService(NewMemoryStore(), manager)
}

func TestCreateAndAuthenticate(t *testing.T) {
	s := newTestService()
	if _, err := s.CreateUser("jane", "jane@example.com", "supersecret", true, false); err != nil {
		t.Fatal(err)
	}

	user, err := s.Authenticate("JANE", "supersecret")
	if err != nil {
		t.Fatalf("case-insensitive login failed: %v", err)
	}
	if !user.IsStaff {
		t.Error("user should be staff")
	}
	if _, err := s.Authenticate("jane", "nope"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Authenticate("ghost", "supersecret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user should give ErrInvalidCredentials, got %v", err)
	}
}

func TestSuperuserImpliesStaff(t *testing.T) {
	s := newTestService()
	user, err := s.CreateUser("root", "", "supersecret", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !user.IsStaff {
		t.Error("superuser should also be staff")
	}
}

func TestInactiveUserCannotAuthenticate(t *testing.T) {
	s := newTestService()
	user, _ := s.CreateUser("jane", "", "supersecret", false, false)
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
	if _, err := s.CreateUser("jane", "", "supersecret", false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser("Jane", "", "supersecret", false, false); !errors.Is(err, ErrUserExists) {
		t.Errorf("got %v, want ErrUserExists", err)
	}
	if _, err := s.CreateUser("ab", "", "supersecret", false, false); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("got %v, want ErrInvalidUser", err)
	}
	if _, err := s.CreateUser("valid", "", "short", false, false); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("got %v, want ErrPasswordTooShort", err)
	}
}

func TestCannotDeleteLastSuperuser(t *testing.T) {
	s := newTestService()
	root, _ := s.CreateUser("root", "", "supersecret", false, true)
	if err := s.Users().Delete(root.ID); !errors.Is(err, ErrLastSuperuser) {
		t.Errorf("got %v, want ErrLastSuperuser", err)
	}
	second, _ := s.CreateUser("root2", "", "supersecret", false, true)
	if err := s.Users().Delete(root.ID); err != nil {
		t.Errorf("deleting one of two superusers should succeed: %v", err)
	}
	if err := s.Users().Delete(second.ID); !errors.Is(err, ErrLastSuperuser) {
		t.Errorf("got %v, want ErrLastSuperuser", err)
	}
}

func TestLoginAndGuards(t *testing.T) {
	s := newTestService()
	manager := s.sessions
	if _, err := s.CreateUser("jane", "", "supersecret", false, false); err != nil {
		t.Fatal(err)
	}

	protected := manager.Middleware(s.Middleware(s.RequireStaff("/login")(
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
		t.Errorf("non-staff got %d, want 403", rec.Code)
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
