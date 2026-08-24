package tests

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/cli"
	"github.com/farhapartex/coyote/core/settings"
)

func messagesProject(t *testing.T) (cli.Context, *bytes.Buffer, string) {
	t.Helper()
	root := t.TempDir()

	write := func(name, body string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("handlers.go", `package main

func handler() {
	l.T("Profile saved.")
	l.N("%d note", "%d notes", 2)
}
`)
	write("templates/pages/home.html", `{{define "content"}}
  <h1>{{.Locale.T "Welcome"}}</h1>
  {{$t := .Locale}}{{range .Items}}{{$t.T "Item"}}{{end}}
{{end}}
`)
	write("handlers_test.go", `package main

func TestIgnored() { l.T("Should not be extracted") }
`)

	cfg, err := settings.New(func(s *settings.Settings) {
		s.Debug = true
		s.SecretKey = strings.Repeat("k", 48)
		s.AllowedHosts = []string{"*"}
		s.BaseDir = root
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en", "fr"},
			Dir:       "locales",
		}
		s.Databases = []settings.Database{{Engine: settings.SQLite, Name: filepath.Join(root, "test.db")}}
	})
	if err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	t.Chdir(root)
	return cli.Context{App: &fakeApp{cfg: cfg}, Out: out}, out, root
}

func TestMakeMessagesWritesATemplateAndCatalogs(t *testing.T) {
	ctx, out, root := messagesProject(t)

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatalf("makemessages: %v", err)
	}

	body := out.String()
	for _, want := range []string{"extracted", "coyote.pot", "fr.po"} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q:\n%s", want, body)
		}
	}

	pot, err := os.ReadFile(filepath.Join(root, "locales", "coyote.pot"))
	if err != nil {
		t.Fatalf("no template written: %v", err)
	}
	for _, want := range []string{
		`msgid "Profile saved."`,
		`msgid "%d note"`,
		`msgid_plural "%d notes"`,
		`msgid "Welcome"`,
		`msgid "Item"`,
		"#: handlers.go:",
		"#: templates/pages/home.html:",
	} {
		if !strings.Contains(string(pot), want) {
			t.Errorf("the template is missing %q:\n%s", want, pot)
		}
	}
	if strings.Contains(string(pot), "Should not be extracted") {
		t.Error("a _test.go file must not be scanned")
	}

	catalog, err := os.ReadFile(filepath.Join(root, "locales", "fr.po"))
	if err != nil {
		t.Fatalf("no French catalog written: %v", err)
	}
	if !strings.Contains(string(catalog), "Language: fr") {
		t.Errorf("fr.po = %s", catalog)
	}
	if _, err := os.Stat(filepath.Join(root, "locales", "en.po")); err == nil {
		t.Error("the default locale needs no catalog")
	}
}

func TestMakeMessagesIsIdempotentAndKeepsTranslations(t *testing.T) {
	ctx, _, root := messagesProject(t)

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "locales", "fr.po")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	translated := strings.Replace(string(original),
		"msgid \"Profile saved.\"\nmsgstr \"\"",
		"msgid \"Profile saved.\"\nmsgstr \"Profil enregistré.\"", 1)
	if translated == string(original) {
		t.Fatalf("could not seed a translation into:\n%s", original)
	}
	if err := os.WriteFile(path, []byte(translated), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "Profil enregistré.") {
		t.Errorf("a re-run lost the translation:\n%s", after)
	}
}

func TestMakeMessagesTurnsARemovedKeyIntoAnObsoleteEntry(t *testing.T) {
	ctx, _, root := messagesProject(t)

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "locales", "fr.po")
	seeded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(seeded),
		"msgid \"Welcome\"\nmsgstr \"\"",
		"msgid \"Welcome\"\nmsgstr \"Bienvenue\"", 1)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	template := filepath.Join(root, "templates", "pages", "home.html")
	if err := os.WriteFile(template, []byte(`{{define "content"}}<h1>nothing</h1>{{end}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), `#~ msgid "Welcome"`) {
		t.Errorf("a removed key should be commented out, not deleted:\n%s", after)
	}
	if !strings.Contains(string(after), "Bienvenue") {
		t.Errorf("the obsolete entry should keep its translation:\n%s", after)
	}
}

func TestMakeMessagesReportsANonConstantMessage(t *testing.T) {
	ctx, out, root := messagesProject(t)

	if err := os.WriteFile(filepath.Join(root, "dynamic.go"), []byte(`package main

func other() { l.T(heading) }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if !strings.Contains(body, "could not be extracted") || !strings.Contains(body, "dynamic.go") {
		t.Errorf("output should name the unextractable call:\n%s", body)
	}
}

func TestCheckMessagesFailsOnAnUntranslatedCatalog(t *testing.T) {
	ctx, out, _ := messagesProject(t)

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	err := (cli.CheckMessages{}).Run(ctx)
	if !errors.Is(err, cli.ErrIncompleteCatalog) {
		t.Fatalf("error = %v, want ErrIncompleteCatalog", err)
	}
	body := out.String()
	if !strings.Contains(body, "untranslated") {
		t.Errorf("output should list untranslated entries:\n%s", body)
	}
	if !strings.Contains(body, "0% translated") {
		t.Errorf("output should report a percentage:\n%s", body)
	}
}

func TestCheckMessagesPassesOnACompleteCatalog(t *testing.T) {
	ctx, out, root := messagesProject(t)

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "locales", "fr.po")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	filled := strings.NewReplacer(
		"msgid \"Profile saved.\"\nmsgstr \"\"", "msgid \"Profile saved.\"\nmsgstr \"Profil enregistré.\"",
		"msgid \"Welcome\"\nmsgstr \"\"", "msgid \"Welcome\"\nmsgstr \"Bienvenue\"",
		"msgid \"Item\"\nmsgstr \"\"", "msgid \"Item\"\nmsgstr \"Article\"",
		"msgstr[0] \"\"\nmsgstr[1] \"\"", "msgstr[0] \"%d note\"\nmsgstr[1] \"%d notes\"",
	).Replace(string(body))
	if err := os.WriteFile(path, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := (cli.CheckMessages{}).Run(ctx); err != nil {
		t.Fatalf("a complete catalog should pass, got %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "100% translated") {
		t.Errorf("output = %s", out.String())
	}
}

func TestCheckMessagesReportsAMissingCatalog(t *testing.T) {
	ctx, out, _ := messagesProject(t)

	err := (cli.CheckMessages{}).Run(ctx)
	if !errors.Is(err, cli.ErrIncompleteCatalog) {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(out.String(), "no catalog") {
		t.Errorf("output = %s", out.String())
	}
}

func TestCheckMessagesStrictFailsOnFuzzyAndObsolete(t *testing.T) {
	ctx, out, root := messagesProject(t)

	if err := (cli.MakeMessages{}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "locales", "fr.po")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	filled := strings.NewReplacer(
		"msgid \"Profile saved.\"\nmsgstr \"\"", "#, fuzzy\nmsgid \"Profile saved.\"\nmsgstr \"Profil\"",
		"msgid \"Welcome\"\nmsgstr \"\"", "msgid \"Welcome\"\nmsgstr \"Bienvenue\"",
		"msgid \"Item\"\nmsgstr \"\"", "msgid \"Item\"\nmsgstr \"Article\"",
		"msgstr[0] \"\"\nmsgstr[1] \"\"", "msgstr[0] \"a\"\nmsgstr[1] \"b\"",
	).Replace(string(body))
	if err := os.WriteFile(path, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := (cli.CheckMessages{Strict: true}).Run(ctx); !errors.Is(err, cli.ErrIncompleteCatalog) {
		t.Errorf("strict mode should fail on a fuzzy entry, got %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "fuzzy") {
		t.Errorf("output should mention the fuzzy entry:\n%s", out.String())
	}
}

func TestCheckMessagesOnlyOneLocale(t *testing.T) {
	ctx, out, _ := messagesProject(t)

	if err := (cli.MakeMessages{Locale: "fr"}).Run(ctx); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	_ = (cli.CheckMessages{Locale: "fr"}).Run(ctx)
	if strings.Count(out.String(), "% translated") > 1 {
		t.Errorf("only one locale should be reported:\n%s", out.String())
	}
}
