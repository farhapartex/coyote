package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
)

func TestSharedRenderDataDoesNotLeakBetweenRequests(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{Username: "alice", Password: "correct horse battery"}); err != nil {
		t.Fatal(err)
	}

	shared := app.Data{"Title": "Home"}
	a.Get("/who", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/whoami.html", shared)
	})

	login := func() *http.Cookie {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/signin", nil)
		a.Get("/signin", func(w http.ResponseWriter, r *http.Request) {
			user, err := a.Auth.Authenticate(t.Context(), "alice", "correct horse battery")
			if err != nil {
				t.Error(err)
				return
			}
			if err := a.Auth.Login(r, user); err != nil {
				t.Error(err)
			}
		})
		a.Handler().ServeHTTP(rec, req)
		for _, c := range rec.Result().Cookies() {
			if strings.Contains(c.Name, "session") {
				return c
			}
		}
		return nil
	}

	cookie := login()
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	signedIn := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/who", nil)
	req.AddCookie(cookie)
	a.Handler().ServeHTTP(signedIn, req)
	t.Logf("signed in  -> %q", strings.TrimSpace(signedIn.Body.String()))

	anonymous := httptest.NewRecorder()
	a.Handler().ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/who", nil))
	t.Logf("anonymous  -> %q", strings.TrimSpace(anonymous.Body.String()))

	if !strings.Contains(signedIn.Body.String(), "user=alice") {
		t.Errorf("the signed-in request should see its own user: %q", signedIn.Body.String())
	}
	if strings.Contains(anonymous.Body.String(), "alice") {
		t.Errorf("an anonymous visitor was shown the signed-in user: %q", anonymous.Body.String())
	}
	if !strings.Contains(anonymous.Body.String(), "user=anonymous") {
		t.Errorf("the anonymous request should see no user: %q", anonymous.Body.String())
	}
	if _, present := shared["User"]; present {
		t.Error("Context must not write into the caller's map")
	}
}
