package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/settings"
)

func localeFS() fstest.MapFS {
	return fstest.MapFS{
		"fr.po": &fstest.MapFile{Data: []byte(frenchPO)},
		"ar.po": &fstest.MapFile{Data: []byte(arabicPO)},
	}
}

func newLocaleApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	base := func(s *settings.Settings) {
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en", "fr", "ar"},
			FS:        localeFS(),
		}
	}
	a := newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)
	a.Get("/where", func(w http.ResponseWriter, r *http.Request) {
		locale := a.Locale(r)
		fmt.Fprintf(w, "%s|%s|%s", locale.Tag(), locale.Direction(), r.URL.Path)
	})
	a.Get("/greet", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, i18n.T(r.Context(), "Save changes"))
	})
	return a
}

func localeRequest(t *testing.T, a *app.App, target string, prepare ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, fn := range prepare {
		fn(req)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func TestLocaleDefaultsWhenNothingSaysOtherwise(t *testing.T) {
	a := newLocaleApp(t)
	if body := localeRequest(t, a, "/where").Body.String(); !strings.HasPrefix(body, "en|ltr|") {
		t.Errorf("body = %q, want the default locale", body)
	}
}

func TestLocaleFromAcceptLanguage(t *testing.T) {
	a := newLocaleApp(t)

	for header, want := range map[string]string{
		"fr":                        "fr",
		"fr-CH, fr;q=0.9, en;q=0.8": "fr",
		"en-GB,en;q=0.9":            "en",
		"ar,en;q=0.5":               "ar",
		"de,it;q=0.9":               "en",
		"":                          "en",
	} {
		rec := localeRequest(t, a, "/where", func(r *http.Request) {
			if header != "" {
				r.Header.Set("Accept-Language", header)
			}
		})
		if got := strings.Split(rec.Body.String(), "|")[0]; got != want {
			t.Errorf("Accept-Language %q gave %q, want %q", header, got, want)
		}
	}
}

func TestLocaleQualityValuesAreRespected(t *testing.T) {
	a := newLocaleApp(t)

	rec := localeRequest(t, a, "/where", func(r *http.Request) {
		r.Header.Set("Accept-Language", "de;q=1.0, fr;q=0.9, ar;q=0.95")
	})
	if got := strings.Split(rec.Body.String(), "|")[0]; got != "ar" {
		t.Errorf("locale = %q, want ar to win on q value", got)
	}
}

func TestLocaleIgnoresAHostileAcceptLanguage(t *testing.T) {
	a := newLocaleApp(t)

	hostile := []string{
		strings.Repeat("fr-FR,", 500) + "fr",
		"fr;q=abc",
		"fr;q=99",
		"*",
		strings.Repeat("x", 400),
		"../../etc/passwd",
		"fr\x00fr",
	}
	for _, header := range hostile {
		rec := localeRequest(t, a, "/where", func(r *http.Request) {
			r.Header.Set("Accept-Language", header)
		})
		if rec.Code != http.StatusOK {
			t.Errorf("header %.20q gave status %d", header, rec.Code)
		}
		tag := strings.Split(rec.Body.String(), "|")[0]
		if tag != "en" && tag != "fr" {
			t.Errorf("header %.20q resolved to %q", header, tag)
		}
	}
}

func TestLocaleFromCookieBeatsTheHeader(t *testing.T) {
	a := newLocaleApp(t)

	rec := localeRequest(t, a, "/where", func(r *http.Request) {
		r.Header.Set("Accept-Language", "ar")
		r.AddCookie(&http.Cookie{Name: "coyote_locale", Value: "fr"})
	})
	if got := strings.Split(rec.Body.String(), "|")[0]; got != "fr" {
		t.Errorf("locale = %q, want the cookie to win", got)
	}
}

func TestLocaleIgnoresAnUnsupportedCookie(t *testing.T) {
	a := newLocaleApp(t)

	for _, value := range []string{"de", "../../etc", "", strings.Repeat("z", 100)} {
		rec := localeRequest(t, a, "/where", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "coyote_locale", Value: value})
		})
		if got := strings.Split(rec.Body.String(), "|")[0]; got != "en" {
			t.Errorf("cookie %q resolved to %q, want the default", value, got)
		}
	}
}

func TestLocaleTranslatesThroughTheRequestContext(t *testing.T) {
	a := newLocaleApp(t)

	rec := localeRequest(t, a, "/greet", func(r *http.Request) {
		r.Header.Set("Accept-Language", "fr")
	})
	if body := rec.Body.String(); body != "Enregistrer les modifications" {
		t.Errorf("body = %q", body)
	}
}

func TestLocaleSetsVaryOnAcceptLanguage(t *testing.T) {
	a := newLocaleApp(t)
	rec := localeRequest(t, a, "/where")

	if vary := rec.Header().Get("Vary"); !strings.Contains(vary, "Accept-Language") {
		t.Errorf("Vary = %q, want it to name Accept-Language", vary)
	}
}

func TestLocaleSetsNoVaryForASingleLocaleProject(t *testing.T) {
	a := newTestApp(t)
	a.Get("/plain", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })

	rec := localeRequest(t, a, "/plain")
	if strings.Contains(rec.Header().Get("Vary"), "Accept-Language") {
		t.Error("a monolingual project should not vary on Accept-Language")
	}
}

func TestLocaleURLPrefixRoutesToTheSameHandler(t *testing.T) {
	a := newLocaleApp(t, func(s *settings.Settings) {
		s.I18N.URLPrefix = true
	})

	rec := localeRequest(t, a, "/fr/where")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want the prefix stripped and the route matched", rec.Code)
	}
	parts := strings.Split(rec.Body.String(), "|")
	if parts[0] != "fr" {
		t.Errorf("locale = %q, want fr", parts[0])
	}
	if parts[2] != "/where" {
		t.Errorf("the handler saw %q, want the prefix stripped", parts[2])
	}
}

func TestLocaleURLPrefixRedirectsTheDefaultLocale(t *testing.T) {
	a := newLocaleApp(t, func(s *settings.Settings) {
		s.I18N.URLPrefix = true
	})

	rec := localeRequest(t, a, "/en/where?page=2")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301 to the unprefixed path", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/where?page=2" {
		t.Errorf("Location = %q, want the query preserved", location)
	}
}

func TestLocaleURLPrefixLeavesUnknownSegmentsAlone(t *testing.T) {
	a := newLocaleApp(t, func(s *settings.Settings) {
		s.I18N.URLPrefix = true
	})

	rec := localeRequest(t, a, "/where")
	if rec.Code != http.StatusOK {
		t.Errorf("an unprefixed path should still serve the default locale, got %d", rec.Code)
	}
	if rec := localeRequest(t, a, "/de/where"); rec.Code != http.StatusNotFound {
		t.Errorf("an unsupported prefix should 404 rather than being swallowed, got %d", rec.Code)
	}
}

func TestLocalePathAndSwitch(t *testing.T) {
	a := newLocaleApp(t, func(s *settings.Settings) {
		s.I18N.URLPrefix = true
	})
	bundle := a.Bundle()

	english := bundle.Locale("en")
	french := bundle.Locale("fr")

	if got := english.Path("/about"); got != "/about" {
		t.Errorf("the default locale should not be prefixed, got %q", got)
	}
	if got := french.Path("/about"); got != "/fr/about" {
		t.Errorf("Path = %q", got)
	}
	if got := french.Switch("/fr/about", "ar"); got != "/ar/about" {
		t.Errorf("Switch = %q", got)
	}
	if got := french.Switch("/fr/about", "en"); got != "/about" {
		t.Errorf("switching to the default should drop the prefix, got %q", got)
	}
	if got := french.Switch("/fr/about", "de"); got != "/fr/about" {
		t.Errorf("an unsupported target should change nothing, got %q", got)
	}
}

func TestLocaleSwitchHandlerSetsTheCookie(t *testing.T) {
	a := newLocaleApp(t, withoutCSRF)
	a.Post("/locale", a.Locales().SwitchHandler("/"))

	form := strings.NewReader("locale=fr&next=/where")
	req := httptest.NewRequest(http.MethodPost, "/locale", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/where" {
		t.Errorf("Location = %q", location)
	}

	found := false
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "coyote_locale" && cookie.Value == "fr" {
			found = true
			if !cookie.HttpOnly || cookie.Path != "/" {
				t.Errorf("cookie = %+v", cookie)
			}
		}
	}
	if !found {
		t.Errorf("no locale cookie was set: %+v", rec.Result().Cookies())
	}
}

func TestLocaleSwitchHandlerWorksWithTheDefaultCSRFGuard(t *testing.T) {
	a := newLocaleApp(t)
	a.Post("/locale", a.Locales().SwitchHandler("/"))
	a.Get("/picker", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<input name="csrf_token" value="` + a.Sessions.CSRFToken(r) + `">`))
	})

	c := newClient(t, a.Handler())
	rec := c.do(http.MethodPost, "/locale", url.Values{
		"csrf_token": {c.token("/picker")},
		"locale":     {"fr"},
		"next":       {"/where"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; the documented picker carries a token", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/where" {
		t.Errorf("Location = %q", location)
	}
}

func TestLocaleSwitchHandlerRefusesAnOffsiteRedirect(t *testing.T) {
	a := newLocaleApp(t, withoutCSRF)
	a.Post("/locale", a.Locales().SwitchHandler("/safe"))

	for _, next := range []string{"//evil.test/x", "https://evil.test/x", "evil"} {
		form := strings.NewReader("locale=fr&next=" + next)
		req := httptest.NewRequest(http.MethodPost, "/locale", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)

		if location := rec.Header().Get("Location"); location != "/safe" {
			t.Errorf("next=%q redirected to %q, want the fallback", next, location)
		}
	}
}

func TestLocaleSwitchHandlerIgnoresAnUnsupportedLocale(t *testing.T) {
	a := newLocaleApp(t)
	a.Post("/locale", a.Locales().SwitchHandler("/"))

	form := strings.NewReader("locale=de&next=/where")
	req := httptest.NewRequest(http.MethodPost, "/locale", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "coyote_locale" {
			t.Errorf("an unsupported locale should set no cookie, got %q", cookie.Value)
		}
	}
}

func TestAcceptLanguageParsing(t *testing.T) {
	got := i18n.ParseAcceptLanguage("en-GB;q=0.8, fr;q=0.9, de")
	want := []string{"de", "fr", "en-GB"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMatchPrefersAnExactTagThenTheBase(t *testing.T) {
	supported := []string{"en", "fr", "pt-BR"}

	for header, want := range map[string]string{
		"pt-BR": "pt-BR",
		"pt":    "pt-BR",
		"pt-PT": "pt-BR",
		"fr-CA": "fr",
		"nl":    "",
	} {
		if got := i18n.Match([]string{header}, supported); got != want {
			t.Errorf("Match(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestLocaleSwitchHandlerReadsNextFromTheQueryToo(t *testing.T) {
	a := newLocaleApp(t)
	a.Get("/locale", a.Locales().SwitchHandler("/"))

	req := httptest.NewRequest(http.MethodGet, "/locale?locale=fr&next=/about", nil)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/about" {
		t.Errorf("Location = %q; a picker rendered as a link carries next in the query, and a "+
			"cacheable page cannot use a form, so the link form has to honour it", location)
	}

	found := false
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "coyote_locale" && cookie.Value == "fr" {
			found = true
		}
	}
	if !found {
		t.Errorf("no locale cookie was set: %+v", rec.Result().Cookies())
	}
}

func TestLocaleSwitchHandlerPrefersThePostedNext(t *testing.T) {
	a := newLocaleApp(t, withoutCSRF)
	a.Post("/locale", a.Locales().SwitchHandler("/"))

	form := strings.NewReader("locale=fr&next=/from-the-form")
	req := httptest.NewRequest(http.MethodPost, "/locale?next=/from-the-query", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if location := rec.Header().Get("Location"); location != "/from-the-form" {
		t.Errorf("Location = %q, want the posted value to win over the query", location)
	}
}

func TestLocaleSwitchHandlerStillRefusesAnOffsiteQueryNext(t *testing.T) {
	a := newLocaleApp(t)
	a.Get("/locale", a.Locales().SwitchHandler("/safe"))

	for _, next := range []string{"//evil.test/x", "https://evil.test/x", "evil"} {
		req := httptest.NewRequest(http.MethodGet,
			"/locale?locale=fr&next="+url.QueryEscape(next), nil)
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)

		if location := rec.Header().Get("Location"); location != "/safe" {
			t.Errorf("next=%q redirected to %q, want the fallback", next, location)
		}
	}
}
