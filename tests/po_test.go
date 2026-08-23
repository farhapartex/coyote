package tests

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/core/i18n"
)

const frenchPO = `# French translation
msgid ""
msgstr ""
"Language: fr\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"
"Content-Type: text/plain; charset=UTF-8\n"

#: templates/pages/home.html:4
msgid "Save changes"
msgstr "Enregistrer les modifications"

msgid "%d note"
msgid_plural "%d notes"
msgstr[0] "%d note"
msgstr[1] "%d notes"

msgctxt "verb"
msgid "Post"
msgstr "Publier"

msgctxt "noun"
msgid "Post"
msgstr "Article"

#, fuzzy
msgid "Delete"
msgstr "Supprimer maybe"

msgid "Multi"
msgstr ""
"one "
"two "
"three"

msgid "Escaped"
msgstr "a\tb\nc\"d\\e"

#~ msgid "Removed long ago"
#~ msgstr "Retiré"
`

func parseFrench(t *testing.T) i18n.Catalog {
	t.Helper()
	catalog, err := i18n.ParsePO("fr", strings.NewReader(frenchPO))
	if err != nil {
		t.Fatalf("ParsePO: %v", err)
	}
	return catalog
}

func TestPOParsesHeadersAndPlurals(t *testing.T) {
	catalog := parseFrench(t)

	if catalog.Tag() != "fr" {
		t.Errorf("Tag = %q", catalog.Tag())
	}
	if catalog.Plurals() != 2 {
		t.Errorf("Plurals = %d, want 2", catalog.Plurals())
	}
	if got := catalog.Header("Language"); got != "fr" {
		t.Errorf("Header(Language) = %q", got)
	}
	if got := catalog.Header("plural-forms"); !strings.Contains(got, "nplurals=2") {
		t.Errorf("Header(Plural-Forms) = %q", got)
	}
}

func TestPOLooksUpASingular(t *testing.T) {
	catalog := parseFrench(t)

	value, found := catalog.Lookup("", "Save changes")
	if !found {
		t.Fatal("Save changes should be translated")
	}
	if value != "Enregistrer les modifications" {
		t.Errorf("value = %q", value)
	}
	if _, found := catalog.Lookup("", "Never seen"); found {
		t.Error("an absent msgid must not be found")
	}
}

func TestPOPluralFormsFollowTheHeaderRule(t *testing.T) {
	catalog := parseFrench(t)

	one, found := catalog.LookupPlural("", "%d note", 1)
	if !found || one != "%d note" {
		t.Errorf("n=1 gave %q, %v", one, found)
	}
	many, found := catalog.LookupPlural("", "%d note", 5)
	if !found || many != "%d notes" {
		t.Errorf("n=5 gave %q, %v", many, found)
	}
	if zero, _ := catalog.LookupPlural("", "%d note", 0); zero != "%d note" {
		t.Errorf("French treats 0 as singular, got %q", zero)
	}
}

func TestPOKeepsContextsApart(t *testing.T) {
	catalog := parseFrench(t)

	verb, _ := catalog.Lookup("verb", "Post")
	noun, _ := catalog.Lookup("noun", "Post")
	if verb != "Publier" || noun != "Article" {
		t.Errorf("verb = %q, noun = %q", verb, noun)
	}
	if _, found := catalog.Lookup("", "Post"); found {
		t.Error("a contextless lookup must not match a msgctxt entry")
	}
}

func TestPOSkipsFuzzyAndObsoleteEntries(t *testing.T) {
	catalog := parseFrench(t)

	if value, found := catalog.Lookup("", "Delete"); found {
		t.Errorf("a fuzzy entry must not be used, got %q", value)
	}
	if _, found := catalog.Lookup("", "Removed long ago"); found {
		t.Error("an obsolete entry must not be loaded")
	}
}

func TestPOJoinsContinuationLines(t *testing.T) {
	catalog := parseFrench(t)

	if value, _ := catalog.Lookup("", "Multi"); value != "one two three" {
		t.Errorf("value = %q, want the joined string", value)
	}
}

func TestPOExpandsEscapes(t *testing.T) {
	catalog := parseFrench(t)

	value, _ := catalog.Lookup("", "Escaped")
	if value != "a\tb\nc\"d\\e" {
		t.Errorf("value = %q", value)
	}
}

func TestPOAcceptsCRLFAndABOM(t *testing.T) {
	source := "\ufeff" + strings.ReplaceAll(frenchPO, "\n", "\r\n")
	catalog, err := i18n.ParsePO("fr", strings.NewReader(source))
	if err != nil {
		t.Fatalf("ParsePO: %v", err)
	}
	if value, found := catalog.Lookup("", "Save changes"); !found || value != "Enregistrer les modifications" {
		t.Errorf("value = %q, %v", value, found)
	}
}

func TestPOAnEmptyTranslationIsAMiss(t *testing.T) {
	catalog, err := i18n.ParsePO("fr", strings.NewReader(`
msgid "Untranslated"
msgstr ""
`))
	if err != nil {
		t.Fatal(err)
	}
	if _, found := catalog.Lookup("", "Untranslated"); found {
		t.Error("an empty msgstr means untranslated, not translated to nothing")
	}
}

func TestPORejectsMalformedInput(t *testing.T) {
	for name, source := range map[string]string{
		"unterminated string": "msgid \"open\nmsgstr \"x\"\n",
		"dangling string":     "\"orphan\"\nmsgid \"a\"\n",
		"unknown keyword":     "msgfoo \"a\"\nmsgstr \"b\"\n",
		"bad plural index":    "msgid \"a\"\nmsgstr[99] \"b\"\n",
		"keyword with no arg": "msgid\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := i18n.ParsePO("fr", strings.NewReader(source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !errors.Is(err, i18n.ErrMalformedPO) {
				t.Errorf("error = %v, want ErrMalformedPO", err)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("the error should name the line: %v", err)
			}
		})
	}
}

func TestFSLoaderReadsAndFallsBackToTheBaseLanguage(t *testing.T) {
	files := fstest.MapFS{
		"locales/fr.po": &fstest.MapFile{Data: []byte(frenchPO)},
	}
	loader := i18n.NewFSLoader(files, "locales")

	catalog, err := loader.Load("fr")
	if err != nil {
		t.Fatalf("Load(fr): %v", err)
	}
	if catalog.Tag() != "fr" {
		t.Errorf("Tag = %q", catalog.Tag())
	}

	regional, err := loader.Load("fr-CA")
	if err != nil {
		t.Fatalf("Load(fr-CA) should fall back to fr.po: %v", err)
	}
	if value, _ := regional.Lookup("", "Save changes"); value != "Enregistrer les modifications" {
		t.Errorf("value = %q", value)
	}

	if _, err := loader.Load("de"); !errors.Is(err, i18n.ErrNoCatalog) {
		t.Errorf("Load(de) = %v, want ErrNoCatalog", err)
	}
}
