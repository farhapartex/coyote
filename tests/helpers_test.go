package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/farhapartex/coyote/contrib/migrate"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
)

const testSecretKey = "test-secret-key-that-is-long-enough-to-pass"

var csrfPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func templateFS() fstest.MapFS {
	return fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{Data: []byte(
			`{{define "base.html"}}<html><body>{{block "content" .}}{{end}}</body></html>{{end}}`)},
		"pages/hello.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<h1>{{.Name}}</h1><p>{{.Path}}</p>{{end}}`)},
		"pages/link.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<a href="{{url "post.detail" 7}}">post</a>{{end}}`)},
		"pages/nonce.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<style nonce="{{.Nonce}}">body{color:red}</style>{{end}}`)},
		"pages/badlink.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<a href="{{url "does.not.exist"}}">x</a>{{end}}`)},
		"pages/whoami.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}user={{with .User}}{{.Username}}{{else}}anonymous{{end}}{{end}}`)},
		"pages/form.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<form method="post">` +
				`<input type="hidden" name="csrf_token" value="{{.CSRFToken}}"></form>{{end}}`)},
	}
}

func devSettings(t *testing.T, fns ...func(*settings.Settings)) settings.Settings {
	t.Helper()
	base := func(s *settings.Settings) {
		s.Debug = true
		s.SecretKey = testSecretKey
		s.AllowedHosts = []string{"*"}
		s.Templates.FS = templateFS()
		s.Templates.Layout = "layouts/base.html"
		s.Auth.PBKDF2Iterations = 1000
		dir := t.TempDir()
		s.BaseDir = dir
		s.Databases = []settings.Database{{
			Engine:       settings.SQLite,
			Name:         filepath.Join(dir, "test.db"),
			MaxOpenConns: 1,
		}}
	}
	resolved, err := settings.New(append([]func(*settings.Settings){base}, fns...)...)
	if err != nil {
		t.Fatalf("building settings: %v", err)
	}
	return resolved
}

func withoutCSRF(s *settings.Settings) { s.Security.CSRF = false }

func prodSettings(fns ...func(*settings.Settings)) []func(*settings.Settings) {
	base := func(s *settings.Settings) {
		s.Debug = false
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"example.com"}
	}
	return append([]func(*settings.Settings){base}, fns...)
}

func newTestApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	a := app.NewFrom(devSettings(t, fns...))
	syncSchema(t, a)
	return a
}

func syncSchema(t *testing.T, a *app.App) {
	t.Helper()
	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	if err := migrate.Sync(handle, a.Models()); err != nil {
		t.Fatalf("syncing schema: %v", err)
	}
}

func newTestManager() *session.Manager {
	return session.NewManager(session.Options{
		Store:    session.NewMemoryStore(0),
		Lifetime: time.Hour,
		HTTPOnly: true,
	})
}

func newTestAuth() (*auth.Service, *session.Manager) {
	manager := newTestManager()
	service := auth.NewService(auth.NewMemoryStore(), manager, auth.Options{
		Hasher:            auth.Hasher{Iterations: 1000},
		MinPasswordLength: 8,
	})
	return service, manager
}

type client struct {
	t       *testing.T
	handler http.Handler
	cookie  *http.Cookie
}

func newClient(t *testing.T, handler http.Handler) *client {
	return &client{t: t, handler: handler}
}

func (c *client) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	c.t.Helper()
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	if cookies := rec.Result().Cookies(); len(cookies) > 0 {
		c.cookie = cookies[0]
	}
	return rec
}

func (c *client) get(target string) *httptest.ResponseRecorder {
	c.t.Helper()
	return c.do(http.MethodGet, target, nil)
}

func (c *client) token(target string) string {
	c.t.Helper()
	body := c.get(target).Body.String()
	match := csrfPattern.FindStringSubmatch(body)
	if match == nil {
		c.t.Fatalf("no csrf token on %s", target)
	}
	return match[1]
}
