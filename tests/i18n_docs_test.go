package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/settings"
)

const docsFrenchPO = `msgid ""
msgstr ""
"Language: fr\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

msgid "Your notes"
msgstr "Vos notes"

msgid "Saved"
msgstr "Enregistré"

msgid "About"
msgstr "À propos"

msgid "%d note"
msgid_plural "%d notes"
msgstr[0] "%d note"
msgstr[1] "%d notes"
`

const docsArabicPO = `msgid ""
msgstr ""
"Language: ar\n"
"Plural-Forms: nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 ? 4 : 5);\n"

msgid "Your notes"
msgstr "ملاحظاتك"

msgid "%d note"
msgid_plural "%d notes"
msgstr[0] "لا ملاحظات"
msgstr[1] "%d"
msgstr[2] "%d"
msgstr[3] "%d"
msgstr[4] "%d"
msgstr[5] "%d"
`

func docsLocaleFS() fstest.MapFS {
	return fstest.MapFS{
		"fr.po": &fstest.MapFile{Data: []byte(docsFrenchPO)},
		"ar.po": &fstest.MapFile{Data: []byte(docsArabicPO)},
	}
}

func documentedTemplates() fstest.MapFS {
	return fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{Data: []byte(
			`{{define "base.html"}}<html lang="{{.Locale.Tag}}" dir="{{.Locale.Direction}}">` +
				`<body>{{block "content" .}}{{end}}</body></html>{{end}}`)},
		"pages/notes.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}` +
				`<h1>{{.Locale.T "Your notes"}}</h1>` +
				`<p>{{.Locale.N "%d note" "%d notes" (len .Notes)}}</p>` +
				`<p>{{.Locale.Tf "Welcome back, %s." .Who}}</p>` +
				`{{$t := .Locale}}{{range .Notes}}<li>{{$t.T "Saved"}} {{.}}</li>{{end}}` +
				`{{end}}`)},
		"pages/links.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<a href="{{.Locale.Path (url "about")}}">{{.Locale.T "About"}}</a>{{end}}`)},
		"pages/picker.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}{{range .Locale.Available}}` +
				`<button name="locale" value="{{.Tag}}" {{if .Active}}disabled{{end}}>{{.Name}}</button>` +
				`{{end}}{{end}}`)},
	}
}

func newDocsApp(t *testing.T, fns ...func(*settings.Settings)) *app.App {
	t.Helper()
	base := func(s *settings.Settings) {
		s.Templates.FS = documentedTemplates()
		s.Templates.Layout = "layouts/base.html"
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en", "fr", "ar"},
			FS:        docsLocaleFS(),
		}
	}
	return newTestApp(t, append([]func(*settings.Settings){base}, fns...)...)
}

func renderIn(t *testing.T, a *app.App, path, page string, data app.Data, tag string) string {
	t.Helper()
	a.Get(path, func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, page, data)
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if tag != "" {
		req.Header.Set("Accept-Language", tag)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("%s rendered %d: %s", page, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func TestDocumentedTemplateSnippetsRender(t *testing.T) {
	a := newDocsApp(t)

	body := renderIn(t, a, "/notes", "pages/notes.html", app.Data{
		"Notes": []string{"first", "second"},
		"Who":   "jane",
	}, "fr")

	for _, want := range []string{
		"Vos notes",
		"2 notes",
		"Welcome back, jane.",
		"<li>Enregistré first</li>",
		`lang="fr"`,
		`dir="ltr"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page is missing %q:\n%s", want, body)
		}
	}
}

func TestDocumentedRangeBindingSeesTheLocale(t *testing.T) {
	a := newDocsApp(t)

	body := renderIn(t, a, "/notes", "pages/notes.html", app.Data{
		"Notes": []string{"x"},
		"Who":   "ann",
	}, "fr")

	if strings.Contains(body, "no locale") || !strings.Contains(body, "Enregistré x") {
		t.Errorf("the $t binding inside range did not resolve:\n%s", body)
	}
}

func TestDocumentedLocalePathPrefixesLinks(t *testing.T) {
	a := newDocsApp(t, func(s *settings.Settings) { s.I18N.URLPrefix = true })
	a.Get("/about", func(w http.ResponseWriter, r *http.Request) {}).Named("about")

	english := renderIn(t, a, "/links", "pages/links.html", app.Data{}, "en")
	if !strings.Contains(english, `href="/about"`) {
		t.Errorf("the default locale should not be prefixed:\n%s", english)
	}

	french := renderIn(t, a, "/fr/links", "pages/links.html", app.Data{}, "")
	if !strings.Contains(french, `href="/fr/about"`) {
		t.Errorf("a non-default locale should be prefixed:\n%s", french)
	}
}

func TestDocumentedPickerListsEveryLocale(t *testing.T) {
	a := newDocsApp(t)

	body := renderIn(t, a, "/picker", "pages/picker.html", app.Data{}, "ar")
	for _, want := range []string{
		`value="en"`, `value="fr"`, `value="ar"`,
		"Français", "العربية",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("picker is missing %q:\n%s", want, body)
		}
	}
	if strings.Count(body, "disabled") != 1 {
		t.Errorf("exactly one locale should be marked active:\n%s", body)
	}
}

func TestDocumentedRTLDirection(t *testing.T) {
	a := newDocsApp(t)

	body := renderIn(t, a, "/notes", "pages/notes.html", app.Data{"Notes": []string{}, "Who": "x"}, "ar")
	if !strings.Contains(body, `dir="rtl"`) || !strings.Contains(body, `lang="ar"`) {
		t.Errorf("an Arabic request should render rtl:\n%s", body)
	}
	if !strings.Contains(body, "لا ملاحظات") {
		t.Errorf("the Arabic zero form should be used:\n%s", body)
	}
	if !strings.Contains(body, "ملاحظاتك") {
		t.Errorf("the Arabic heading should be translated:\n%s", body)
	}
}

func TestDocumentedMonolingualProjectStillRenders(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Templates.FS = documentedTemplates()
		s.Templates.Layout = "layouts/base.html"
	})

	body := renderIn(t, a, "/notes", "pages/notes.html", app.Data{
		"Notes": []string{"a"},
		"Who":   "sam",
	}, "")

	for _, want := range []string{"Your notes", "1 note", "Welcome back, sam.", `lang="en"`} {
		if !strings.Contains(body, want) {
			t.Errorf("a project with no catalogs is missing %q:\n%s", want, body)
		}
	}
}
