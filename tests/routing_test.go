package tests

import (
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/certs"
	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/router"
	"github.com/farhapartex/coyote/core/settings"
)

func noop(w http.ResponseWriter, r *http.Request) {}

func TestRoutesCanBeNamed(t *testing.T) {
	a := newTestApp(t)
	a.Get("/", noop).Named("home")
	a.Get("/posts/{id}", noop).Named("post.detail")

	found, ok := a.Named("post.detail")
	if !ok {
		t.Fatal("named route not found")
	}
	if found.Pattern != "/posts/{id}" || found.Method != http.MethodGet {
		t.Errorf("unexpected route: %+v", found)
	}
	if _, ok := a.Named("nope"); ok {
		t.Error("an unknown name should not resolve")
	}

	names := map[string]bool{}
	for _, route := range a.Routes() {
		if route.Name != "" {
			names[route.Name] = true
		}
	}
	if !names["home"] || !names["post.detail"] {
		t.Errorf("names missing from the route table: %v", names)
	}
}

func TestNamingIsOptional(t *testing.T) {
	a := newTestApp(t)
	a.Get("/anonymous", noop)
	for _, route := range a.Routes() {
		if route.Pattern == "/anonymous" && route.Name != "" {
			t.Errorf("unnamed route picked up a name: %q", route.Name)
		}
	}
}

func TestDuplicateNamesPanic(t *testing.T) {
	a := newTestApp(t)
	a.Get("/one", noop).Named("shared")

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a duplicate name should panic at registration")
		}
		err, ok := recovered.(error)
		if !ok || !errors.Is(err, router.ErrDuplicateName) {
			t.Errorf("got %v, want ErrDuplicateName", recovered)
		}
	}()
	a.Get("/two", noop).Named("shared")
}

func TestReverse(t *testing.T) {
	a := newTestApp(t)
	a.Get("/{$}", noop).Named("home")
	a.Get("/posts/{id}", noop).Named("post.detail")
	a.Get("/shops/{shop}/items/{item}", noop).Named("item.detail")
	a.Get("/files/{path...}", noop).Named("file")

	cases := []struct {
		name   string
		values []any
		want   string
	}{
		{"home", nil, "/"},
		{"post.detail", []any{42}, "/posts/42"},
		{"post.detail", []any{"hello world"}, "/posts/hello%20world"},
		{"item.detail", []any{7, "blue-shoe"}, "/shops/7/items/blue-shoe"},
		{"file", []any{"docs/readme.md"}, "/files/docs/readme.md"},
	}
	for _, c := range cases {
		got, err := a.Reverse(c.name, c.values...)
		if err != nil {
			t.Errorf("Reverse(%q, %v): %v", c.name, c.values, err)
			continue
		}
		if got != c.want {
			t.Errorf("Reverse(%q, %v) = %q, want %q", c.name, c.values, got, c.want)
		}
	}
}

func TestReverseErrors(t *testing.T) {
	a := newTestApp(t)
	a.Get("/posts/{id}", noop).Named("post.detail")

	if _, err := a.Reverse("missing"); !errors.Is(err, router.ErrUnknownName) {
		t.Errorf("got %v, want ErrUnknownName", err)
	}
	if _, err := a.Reverse("post.detail"); !errors.Is(err, router.ErrWrongArity) {
		t.Errorf("too few values: got %v, want ErrWrongArity", err)
	}
	if _, err := a.Reverse("post.detail", 1, 2); !errors.Is(err, router.ErrWrongArity) {
		t.Errorf("too many values: got %v, want ErrWrongArity", err)
	}
	if _, err := a.Reverse("post.detail", ""); !errors.Is(err, router.ErrWrongArity) {
		t.Errorf("empty value: got %v, want ErrWrongArity", err)
	}
}

func TestReverseRoundTripsThroughTheRouter(t *testing.T) {
	a := newTestApp(t)
	a.Get("/shops/{shop}/items/{item}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.PathValue("shop") + "/" + r.PathValue("item")))
	}).Named("item.detail")

	path := a.MustReverse("item.detail", 7, "blue-shoe")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("a reversed URL should route back: %d", rec.Code)
	}
	if rec.Body.String() != "7/blue-shoe" {
		t.Errorf("wildcards did not survive the round trip: %q", rec.Body.String())
	}
}

func TestNamedRoutesInsideGroups(t *testing.T) {
	a := newTestApp(t)
	api := a.Group("/api")
	api.Get("/status/{code}", noop).Named("api.status")

	got, err := a.Reverse("api.status", 200)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/status/200" {
		t.Errorf("Reverse = %q, want /api/status/200", got)
	}
}

func TestURLTemplateFunction(t *testing.T) {
	a := newTestApp(t)
	a.Get("/posts/{id}", noop).Named("post.detail")
	a.Get("/link", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/link.html", nil)
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/link", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `href="/posts/7"`) {
		t.Errorf("the url function did not resolve: %s", rec.Body.String())
	}
}

func TestURLTemplateFunctionReportsBadNames(t *testing.T) {
	a := newTestApp(t)
	a.Get("/broken", func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/badlink.html", nil)
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/broken", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("an unknown route name should fail the render, got %d", rec.Code)
	}
}

func TestTLSSettings(t *testing.T) {
	plain := settings.TLS{}
	if plain.Enabled() || plain.Scheme() != "http" {
		t.Error("TLS should be off by default")
	}

	pair := settings.TLS{CertFile: "cert.pem", KeyFile: "key.pem"}
	if !pair.Enabled() || pair.Scheme() != "https" {
		t.Error("a cert and key should enable TLS")
	}
	if pair.Build().MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want TLS 1.2", pair.Build().MinVersion)
	}

	custom := settings.TLS{Config: &tls.Config{MinVersion: tls.VersionTLS13}}
	if !custom.Enabled() {
		t.Error("a supplied tls.Config should enable TLS")
	}
	if custom.Build().MinVersion != tls.VersionTLS13 {
		t.Error("a supplied MinVersion should be kept")
	}
	if custom.Build() == custom.Config {
		t.Error("Build should clone, not hand out the original")
	}
}

func TestTLSValidation(t *testing.T) {
	cases := map[string]func(*settings.Settings){
		"CertFile is set without": func(s *settings.Settings) { s.Server.TLS.CertFile = "cert.pem" },
		"KeyFile is set without":  func(s *settings.Settings) { s.Server.TLS.KeyFile = "key.pem" },
		"cannot be negative":      func(s *settings.Settings) { s.Server.TLS.HSTS = -time.Hour },
		"only makes sense":        func(s *settings.Settings) { s.Server.TLS.HSTS = time.Hour },
	}
	for want, mutate := range cases {
		_, err := settings.New(prodSettings(mutate)...)
		if err == nil {
			t.Errorf("expected an error for %q", want)
			continue
		}
		mustContain(t, problemsOf(t, err), want)
	}

	if _, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Server.TLS.CertFile = "cert.pem"
		s.Server.TLS.KeyFile = "key.pem"
		s.Server.TLS.HSTS = 30 * 24 * time.Hour
	})...); err != nil {
		t.Errorf("a complete TLS config should validate: %v", err)
	}
}

func TestSchemeAndBaseURL(t *testing.T) {
	plain, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Scheme() != "http" || !strings.HasPrefix(plain.BaseURL(), "http://") {
		t.Errorf("BaseURL = %q", plain.BaseURL())
	}

	secure, err := settings.New(prodSettings(func(s *settings.Settings) {
		s.Server.TLS.CertFile = "cert.pem"
		s.Server.TLS.KeyFile = "key.pem"
	})...)
	if err != nil {
		t.Fatal(err)
	}
	if secure.Scheme() != "https" || !strings.HasPrefix(secure.BaseURL(), "https://") {
		t.Errorf("BaseURL = %q", secure.BaseURL())
	}
}

func TestHSTSOnlyOverSecureConnections(t *testing.T) {
	handler := middleware.HSTS(48*time.Hour, 1)(http.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("plain HTTP should not carry HSTS, got %q", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=172800") {
		t.Errorf("HSTS = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("a direct TLS connection should carry HSTS")
	}
}

func TestRequireHTTPS(t *testing.T) {
	handler := middleware.RequireHTTPS(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secure"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pay?x=1", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("plain HTTP should redirect, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "https://example.com/pay?x=1" {
		t.Errorf("Location = %q", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/pay", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "secure" {
		t.Errorf("a proxied HTTPS request should pass through: %d %q", rec.Code, rec.Body.String())
	}
}

func TestServerConfigureHook(t *testing.T) {
	called := false
	resolved := devSettings(t, func(s *settings.Settings) {
		s.Server.Configure = func(server *http.Server) {
			called = true
			server.MaxHeaderBytes = 4096
		}
	})
	if resolved.Server.Configure == nil {
		t.Fatal("the hook should survive validation")
	}
	server := &http.Server{}
	resolved.Server.Configure(server)
	if !called || server.MaxHeaderBytes != 4096 {
		t.Error("the hook did not run")
	}
}

func autocertSettings(t *testing.T, fns ...func(*settings.Settings)) []func(*settings.Settings) {
	t.Helper()
	base := func(s *settings.Settings) {
		settings.Production.Apply(s)
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"example.com", "www.example.com"}
		s.Server.Port = 443
		s.Server.TLS.Autocert = true
		s.Server.TLS.AcceptTOS = true
		s.BaseDir = t.TempDir()
	}
	return append([]func(*settings.Settings){base}, fns...)
}

func TestAutocertEnablesTLS(t *testing.T) {
	resolved, err := settings.New(autocertSettings(t)...)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Server.TLS.Enabled() || !resolved.Server.TLS.Managed() {
		t.Error("autocert should enable and manage TLS")
	}
	if resolved.Scheme() != "https" {
		t.Errorf("Scheme = %q", resolved.Scheme())
	}
	if !filepath.IsAbs(resolved.Server.TLS.CacheDir) {
		t.Errorf("CacheDir should resolve against BaseDir, got %q", resolved.Server.TLS.CacheDir)
	}
	if filepath.Base(resolved.Server.TLS.CacheDir) != "certs" {
		t.Errorf("CacheDir = %q, want a certs directory", resolved.Server.TLS.CacheDir)
	}
}

func TestAutocertValidation(t *testing.T) {
	cases := map[string]func(*settings.Settings){
		"AcceptTOS": func(s *settings.Settings) { s.Server.TLS.AcceptTOS = false },
		"cannot be combined with CertFile": func(s *settings.Settings) {
			s.Server.TLS.CertFile = "cert.pem"
			s.Server.TLS.KeyFile = "key.pem"
		},
		"needs AllowedHosts":      func(s *settings.Settings) { s.AllowedHosts = nil },
		"cannot use the \"*\"":    func(s *settings.Settings) { s.AllowedHosts = []string{"*"} },
		"Server.Port must be 443": func(s *settings.Settings) { s.Server.Port = 8443 },
		"supplied Config":         func(s *settings.Settings) { s.Server.TLS.Config = &tls.Config{} },
	}
	for want, mutate := range cases {
		_, err := settings.New(autocertSettings(t, mutate)...)
		if err == nil {
			t.Errorf("expected an error mentioning %q", want)
			continue
		}
		mustContain(t, problemsOf(t, err), want)
	}
}

func TestAutocertTLSConfig(t *testing.T) {
	resolved, err := settings.New(autocertSettings(t)...)
	if err != nil {
		t.Fatal(err)
	}
	config, err := certs.TLSConfig(resolved.Server.TLS, resolved.AllowedHosts)
	if err != nil {
		t.Fatal(err)
	}
	if config.GetCertificate == nil {
		t.Error("a managed config must fetch certificates on demand")
	}
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x", config.MinVersion)
	}

	protocols := strings.Join(config.NextProtos, ",")
	for _, want := range []string{"h2", "acme-tls/1"} {
		if !strings.Contains(protocols, want) {
			t.Errorf("NextProtos %q is missing %q", protocols, want)
		}
	}
	if _, err := os.Stat(resolved.Server.TLS.CacheDir); err != nil {
		t.Errorf("the cache directory should exist: %v", err)
	}
}

func TestAutocertStagingDirectory(t *testing.T) {
	resolved, err := settings.New(autocertSettings(t, func(s *settings.Settings) {
		s.Server.TLS.Staging = true
	})...)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := certs.Manager(resolved.Server.TLS, resolved.AllowedHosts)
	if err != nil {
		t.Fatal(err)
	}
	if manager.Client == nil || manager.Client.DirectoryURL != certs.StagingDirectory {
		t.Errorf("staging should point at the staging directory, got %+v", manager.Client)
	}

	live, _ := settings.New(autocertSettings(t)...)
	liveManager, err := certs.Manager(live.Server.TLS, live.AllowedHosts)
	if err != nil {
		t.Fatal(err)
	}
	if liveManager.Client != nil {
		t.Error("without Staging the default production directory should be used")
	}
}

func TestAutocertRefusesWithoutHosts(t *testing.T) {
	resolved, _ := settings.New(autocertSettings(t)...)
	if _, err := certs.Manager(resolved.Server.TLS, nil); !errors.Is(err, certs.ErrNoHosts) {
		t.Errorf("got %v, want ErrNoHosts", err)
	}
}

func TestUnmanagedTLSConfigIsUnchanged(t *testing.T) {
	config, err := certs.TLSConfig(settings.TLS{CertFile: "c", KeyFile: "k"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.GetCertificate != nil {
		t.Error("a cert/key pair should not install a certificate fetcher")
	}
}

func TestForwardedProtoIsIgnoredWithNoProxyDeclared(t *testing.T) {
	handler := middleware.RequireHTTPS(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secure"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/pay", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want a redirect; a claim of https means nothing with no proxy in front", rec.Code)
	}
	if rec.Body.String() == "secure" {
		t.Error("the handler ran over plain HTTP on the strength of a header the client wrote")
	}
}

func TestHSTSIsNotSentOnAForgedForwardedProto(t *testing.T) {
	handler := middleware.HSTS(48*time.Hour, 0)(http.HandlerFunc(noop))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS = %q; a forged header must not pin a browser to https", got)
	}
}

type hijackable struct {
	*httptest.ResponseRecorder
	taken bool
}

func (h *hijackable) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.taken = true
	server, client := net.Pipe()
	client.Close()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

func TestTheMiddlewareChainCanBeHijackedForAnUpgrade(t *testing.T) {
	a := newTestApp(t, withoutCSRF, func(s *settings.Settings) {
		s.Security.Compress = true
		s.PageCache = settings.PageCache{Enabled: true, TTL: time.Minute, Paths: []string{"/"}}
	})

	var hijackErr error
	a.Get("/upgrade", func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		hijackErr = err
		if conn != nil {
			conn.Close()
		}
	})

	rec := &hijackable{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/upgrade", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	a.Handler().ServeHTTP(rec, req)

	if !rec.taken {
		t.Error("the handler could not reach the underlying connection through the chain")
	}
	if hijackErr != nil {
		t.Errorf("Hijack: %v", hijackErr)
	}
}
