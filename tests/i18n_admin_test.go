package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/contrib/accounts"
	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func translatedPortal(t *testing.T, extra fstest.MapFS) (*app.App, *client) {
	t.Helper()

	a := newTestApp(t, func(s *settings.Settings) {
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en", "fr", "ar"},
			FS:        extra,
		}
	})
	admin.Mount(a)
	accounts.Mount(a, accounts.Options{AllowRegistration: true})

	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "correct horse battery"); err != nil {
		t.Fatal(err)
	}

	c := newClient(t, a.Handler())
	token := c.token("/admin/login")
	c.do(http.MethodPost, "/admin/login", map[string][]string{
		"csrf_token": {token},
		"username":   {"root"},
		"password":   {"correct horse battery"},
	})
	return a, c
}

func inLocale(t *testing.T, a *app.App, target, tag string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept-Language", tag)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestAdminShipsAFrenchCatalog(t *testing.T) {
	a, c := translatedPortal(t, nil)

	english := c.get("/admin/").Body.String()
	if !strings.Contains(english, "Dashboard") {
		t.Fatalf("English dashboard missing its heading:\n%s", english[:min(400, len(english))])
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.Header.Set("Accept-Language", "fr")
	req.AddCookie(c.cookie)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	french := rec.Body.String()

	for _, want := range []string{"Tableau de bord", "Utilisateurs", "Déconnexion", "Sessions actives"} {
		if !strings.Contains(french, want) {
			t.Errorf("the French portal is missing %q", want)
		}
	}
	if strings.Contains(french, ">Dashboard<") {
		t.Error("the French portal still shows the English heading")
	}
}

func TestAdminFrenchLoginPage(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.I18N = settings.I18N{Default: "en", Supported: []string{"en", "fr"}}
	})
	admin.Mount(a)

	body := inLocale(t, a, "/admin/login", "fr")
	for _, want := range []string{"Nom d’utilisateur", "Mot de passe", "Connexion"} {
		if !strings.Contains(body, want) {
			t.Errorf("the French login page is missing %q:\n%s", want, body)
		}
	}
}

func TestAccountsShipsAFrenchCatalog(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.I18N = settings.I18N{Default: "en", Supported: []string{"en", "fr"}}
	})
	accounts.Mount(a, accounts.Options{AllowRegistration: true})

	body := inLocale(t, a, "/accounts/register", "fr")
	for _, want := range []string{"Créer un compte", "Prénom", "Nom d’utilisateur", "Créer le compte"} {
		if !strings.Contains(body, want) {
			t.Errorf("the French registration page is missing %q:\n%s", want, body)
		}
	}
}

func TestAdminRendersRTLForArabic(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.I18N = settings.I18N{Default: "en", Supported: []string{"en", "ar"}}
	})
	admin.Mount(a)

	arabic := inLocale(t, a, "/admin/login", "ar")
	if !strings.Contains(arabic, `dir="rtl"`) {
		t.Errorf("an Arabic request should render dir=rtl:\n%s", arabic[:min(300, len(arabic))])
	}
	if !strings.Contains(arabic, `lang="ar"`) {
		t.Error("the lang attribute should follow the locale")
	}

	english := inLocale(t, a, "/admin/login", "en")
	if !strings.Contains(english, `dir="ltr"`) || !strings.Contains(english, `lang="en"`) {
		t.Error("an English request should render ltr")
	}
}

func TestAdminStylesheetHasNoDirectionalRules(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "contrib", "admin", "templates", "base.html"))
	if err != nil {
		t.Fatal(err)
	}
	style := string(raw)
	start := strings.Index(style, "<style")
	end := strings.Index(style, "</style>")
	if start < 0 || end < 0 {
		t.Fatal("no stylesheet found")
	}
	style = style[start:end]

	for _, banned := range []string{
		"text-align: left", "text-align: right",
		"margin-left", "margin-right", "padding-left", "padding-right",
		"border-left:", "border-right:", "float: left", "float: right",
	} {
		if strings.Contains(style, banned) {
			t.Errorf("the admin stylesheet uses %q; use a logical property so RTL mirrors", banned)
		}
	}
}

func TestApplicationCatalogOverridesTheFrameworkOne(t *testing.T) {
	own := fstest.MapFS{
		"fr.po": &fstest.MapFile{Data: []byte(`msgid ""
msgstr ""
"Language: fr\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

msgid "Dashboard"
msgstr "Mon tableau"
`)},
	}

	a, c := translatedPortal(t, own)

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.Header.Set("Accept-Language", "fr")
	req.AddCookie(c.cookie)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()

	if !strings.Contains(body, "Mon tableau") {
		t.Errorf("the application catalog should win:\n%s", body[:min(600, len(body))])
	}
	if strings.Contains(body, "Tableau de bord") {
		t.Error("the framework string should have been overridden")
	}
	if !strings.Contains(body, "Utilisateurs") {
		t.Error("strings the application did not override should still come from the framework")
	}
}

func TestAdminPluralFormsInFrench(t *testing.T) {
	a, c := translatedPortal(t, nil)

	if _, err := a.SyncPermissions(t.Context()); err != nil {
		t.Fatal(err)
	}

	store := a.Auth.Permissions()
	role := &auth.Role{Name: "Lecteur"}
	if err := store.CreateRole(t.Context(), role); err != nil {
		t.Fatal(err)
	}
	all, err := store.AllPermissions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Fatalf("expected permissions after a sync, got %d", len(all))
	}
	if err := store.SetRolePermissions(t.Context(), role.ID, []string{all[0].ID}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/roles", nil)
	req.Header.Set("Accept-Language", "fr")
	req.AddCookie(c.cookie)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "1 permission accordée") {
		t.Errorf("the singular French form should be used:\n%s", rec.Body.String())
	}

	if err := store.SetRolePermissions(t.Context(), role.ID, []string{all[0].ID, all[1].ID}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/admin/roles", nil)
	req.Header.Set("Accept-Language", "fr")
	req.AddCookie(c.cookie)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "2 permissions accordées") {
		t.Errorf("the plural French form should be used:\n%s", rec.Body.String())
	}
}

func TestUntranslatedLocaleFallsBackToEnglishInTheAdmin(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.I18N = settings.I18N{Default: "en", Supported: []string{"en", "ar"}}
	})
	admin.Mount(a)

	body := inLocale(t, a, "/admin/login", "ar")
	if !strings.Contains(body, "Log in") {
		t.Errorf("a locale with no catalog should show English, not blanks:\n%s", body)
	}
	if strings.Contains(body, `value=""`) && strings.Contains(body, "<label></label>") {
		t.Error("no label should render empty")
	}
}
