package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/settings"
)

const arabicPO = `msgid ""
msgstr ""
"Language: ar\n"
"Plural-Forms: nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 ? 4 : 5);\n"

msgid "Users"
msgstr "المستخدمون"

msgid "%d note"
msgid_plural "%d notes"
msgstr[0] "لا ملاحظات"
msgstr[1] "ملاحظة واحدة"
msgstr[2] "ملاحظتان"
msgstr[3] "%d ملاحظات"
msgstr[4] "%d ملاحظة"
msgstr[5] "%d ملاحظة"
`

const quebecPO = `msgid ""
msgstr ""
"Language: fr-CA\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

msgid "Save changes"
msgstr "Sauvegarder"
`

func testBundle(t *testing.T, opts i18n.Options, files fstest.MapFS) *i18n.Bundle {
	t.Helper()
	bundle := i18n.NewBundle(opts)
	if problems := i18n.LoadInto(bundle, i18n.NewFSLoader(files, "locales")); len(problems) > 0 {
		for _, problem := range problems {
			t.Logf("loader: %v", problem)
		}
	}
	return bundle
}

func TestLocaleTranslatesAndFallsBackToTheMsgid(t *testing.T) {
	bundle := testBundle(t, i18n.Options{
		Default:   "en",
		Supported: []string{"en", "fr"},
	}, fstest.MapFS{"locales/fr.po": &fstest.MapFile{Data: []byte(frenchPO)}})

	french := bundle.Locale("fr")
	if got := french.T("Save changes"); got != "Enregistrer les modifications" {
		t.Errorf("T = %q", got)
	}
	if got := french.T("Never translated"); got != "Never translated" {
		t.Errorf("a missing key must render its own English, got %q", got)
	}

	english := bundle.Locale("en")
	if got := english.T("Save changes"); got != "Save changes" {
		t.Errorf("the default locale renders the msgid, got %q", got)
	}
}

func TestLocaleNeverReturnsAnEmptyString(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en", "fr"}},
		fstest.MapFS{"locales/fr.po": &fstest.MapFile{Data: []byte(frenchPO)}})

	for _, msgid := range []string{"Untranslated thing", "Delete", "Multi"} {
		if got := bundle.Locale("fr").T(msgid); got == "" {
			t.Errorf("T(%q) returned an empty string", msgid)
		}
	}
}

func TestLocaleFallbackChain(t *testing.T) {
	bundle := testBundle(t, i18n.Options{
		Default:   "en",
		Supported: []string{"en", "fr", "fr-CA"},
		Fallbacks: map[string]string{"fr-CA": "fr"},
	}, fstest.MapFS{
		"locales/fr.po":    &fstest.MapFile{Data: []byte(frenchPO)},
		"locales/fr-CA.po": &fstest.MapFile{Data: []byte(quebecPO)},
	})

	quebec := bundle.Locale("fr-CA")
	if got := quebec.T("Save changes"); got != "Sauvegarder" {
		t.Errorf("the regional catalog should win, got %q", got)
	}
	if got := quebec.TC("verb", "Post"); got != "Publier" {
		t.Errorf("a key absent regionally should come from fr, got %q", got)
	}

	chain := quebec.Chain()
	if len(chain) < 3 || chain[0] != "fr-CA" || chain[1] != "fr" || chain[len(chain)-1] != "en" {
		t.Errorf("chain = %v, want fr-CA then fr then en", chain)
	}
}

func TestLocaleArabicPluralsUseSixForms(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en", "ar"}},
		fstest.MapFS{"locales/ar.po": &fstest.MapFile{Data: []byte(arabicPO)}})

	arabic := bundle.Locale("ar")
	for count, want := range map[int]string{
		0: "لا ملاحظات",
		1: "ملاحظة واحدة",
		2: "ملاحظتان",
		3: "3 ملاحظات",
	} {
		if got := arabic.N("%d note", "%d notes", count); got != want {
			t.Errorf("N(%d) = %q, want %q", count, got, want)
		}
	}
}

func TestLocalePluralFallsBackToEnglishForms(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en"}}, fstest.MapFS{})
	english := bundle.Locale("en")

	if got := english.N("%d note", "%d notes", 1); got != "1 note" {
		t.Errorf("N(1) = %q", got)
	}
	if got := english.N("%d note", "%d notes", 4); got != "4 notes" {
		t.Errorf("N(4) = %q", got)
	}
}

func TestLocaleInterpolatesArguments(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en"}}, fstest.MapFS{})
	locale := bundle.Locale("en")

	if got := locale.Tf("Welcome back, %s.", "jane"); got != "Welcome back, jane." {
		t.Errorf("Tf = %q", got)
	}
	if got := locale.T("No placeholders here"); got != "No placeholders here" {
		t.Errorf("T = %q", got)
	}
}

func TestLocaleReportsAMissingKeyOnceOnly(t *testing.T) {
	var mu sync.Mutex
	reports := []string{}

	bundle := i18n.NewBundle(i18n.Options{
		Default:   "en",
		Supported: []string{"en", "fr"},
		Debug:     true,
		OnMissing: func(tag, msgid string) {
			mu.Lock()
			reports = append(reports, tag+":"+msgid)
			mu.Unlock()
		},
	})
	catalog, err := i18n.ParsePO("fr", strings.NewReader(frenchPO))
	if err != nil {
		t.Fatal(err)
	}
	bundle.Add(catalog)

	locale := bundle.Locale("fr")
	for range 1000 {
		locale.T("Absent key")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reports) != 1 {
		t.Errorf("a thousand renders reported %d times, want 1", len(reports))
	}
	if len(locale.Missing()) != 1 {
		t.Errorf("Missing = %v", locale.Missing())
	}
}

func TestLocaleUnsupportedTagFallsBackToTheDefault(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en", "fr"}},
		fstest.MapFS{"locales/fr.po": &fstest.MapFile{Data: []byte(frenchPO)}})

	if got := bundle.Locale("de").Tag(); got != "en" {
		t.Errorf("Tag = %q, want the default", got)
	}
	if got := bundle.Locale("").Tag(); got != "en" {
		t.Errorf("an empty tag should give the default, got %q", got)
	}
}

func TestNilLocaleStillRenders(t *testing.T) {
	var locale *i18n.Locale

	if got := locale.T("Save changes"); got != "Save changes" {
		t.Errorf("a nil locale must render the msgid, got %q", got)
	}
	if got := locale.N("%d note", "%d notes", 3); got != "3 notes" {
		t.Errorf("N = %q", got)
	}
	if got := locale.Tag(); got != "en" {
		t.Errorf("Tag = %q", got)
	}
	if got := locale.Direction(); got != "ltr" {
		t.Errorf("Direction = %q", got)
	}
}

func TestContextHelpersReadTheLocale(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en", "fr"}},
		fstest.MapFS{"locales/fr.po": &fstest.MapFile{Data: []byte(frenchPO)}})

	ctx := i18n.WithLocale(context.Background(), bundle.Locale("fr"))
	if got := i18n.T(ctx, "Save changes"); got != "Enregistrer les modifications" {
		t.Errorf("i18n.T = %q", got)
	}
	if got := i18n.TC(ctx, "noun", "Post"); got != "Article" {
		t.Errorf("i18n.TC = %q", got)
	}
	if got := i18n.Tag(ctx); got != "fr" {
		t.Errorf("i18n.Tag = %q", got)
	}

	bare := context.Background()
	if got := i18n.T(bare, "Save changes"); got != "Save changes" {
		t.Errorf("a context with no locale must still translate to English, got %q", got)
	}
}

func TestDirectionAndNames(t *testing.T) {
	for tag, want := range map[string]string{
		"ar": "rtl", "he": "rtl", "fa-IR": "rtl", "ur": "rtl",
		"en": "ltr", "fr-CA": "ltr", "ja": "ltr", "": "ltr",
	} {
		if got := i18n.DirectionOf(tag); got != want {
			t.Errorf("DirectionOf(%q) = %q, want %q", tag, got, want)
		}
	}
	if got := i18n.NameOf("fr"); got != "Français" {
		t.Errorf("NameOf(fr) = %q", got)
	}
	if got := i18n.NameOf("fr-CA"); got != "Français" {
		t.Errorf("NameOf(fr-CA) = %q", got)
	}
	if got := i18n.NameOf("xx"); got != "xx" {
		t.Errorf("an unknown tag should return itself, got %q", got)
	}
}

func TestNormaliseTags(t *testing.T) {
	for input, want := range map[string]string{
		"fr":       "fr",
		"FR":       "fr",
		"fr_ca":    "fr-CA",
		"zh-hant":  "zh-Hant",
		"  en-gb ": "en-GB",
		"":         "",
	} {
		if got := i18n.Normalise(input); got != want {
			t.Errorf("Normalise(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLocaleAvailableListsSupportedLocales(t *testing.T) {
	bundle := testBundle(t, i18n.Options{Default: "en", Supported: []string{"en", "fr", "ar"}},
		fstest.MapFS{"locales/fr.po": &fstest.MapFile{Data: []byte(frenchPO)}})

	available := bundle.Locale("fr").Available()
	if len(available) != 3 {
		t.Fatalf("Available = %d entries, want 3", len(available))
	}

	active := 0
	for _, entry := range available {
		if entry.Active {
			active++
			if entry.Tag != "fr" {
				t.Errorf("the active entry is %q", entry.Tag)
			}
		}
		if entry.Tag == "ar" && entry.Direction != "rtl" {
			t.Errorf("Arabic should report rtl, got %q", entry.Direction)
		}
		if entry.Name == "" {
			t.Errorf("%q has no display name", entry.Tag)
		}
	}
	if active != 1 {
		t.Errorf("%d entries marked active, want 1", active)
	}
}

func TestI18NSettingsDefaults(t *testing.T) {
	resolved := settings.Default()
	if resolved.I18N.Locale() != "en" {
		t.Errorf("default locale = %q", resolved.I18N.Locale())
	}
	if resolved.I18N.Cookie() != "coyote_locale" {
		t.Errorf("cookie = %q", resolved.I18N.Cookie())
	}
	if resolved.TimeZone != "UTC" {
		t.Errorf("TimeZone = %q", resolved.TimeZone)
	}
	if resolved.I18N.Enabled() {
		t.Error("one supported locale is not a multilingual project")
	}
}

func TestI18NValidationRejectsBadConfigurations(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*settings.Settings)
		want   string
	}{
		"default not supported": {
			mutate: func(s *settings.Settings) {
				s.I18N = settings.I18N{Default: "de", Supported: []string{"en", "fr"}}
			},
			want: "is not in I18N.Supported",
		},
		"duplicate locale": {
			mutate: func(s *settings.Settings) {
				s.I18N = settings.I18N{Default: "en", Supported: []string{"en", "EN"}}
			},
			want: "more than once",
		},
		"fallback to itself": {
			mutate: func(s *settings.Settings) {
				s.I18N = settings.I18N{Default: "en", Supported: []string{"en"},
					Fallbacks: map[string]string{"fr": "FR"}}
			},
			want: "to itself",
		},
		"unknown time zone": {
			mutate: func(s *settings.Settings) { s.TimeZone = "Mars/Olympus_Mons" },
			want:   "cannot be loaded",
		},
		"empty locale entry": {
			mutate: func(s *settings.Settings) {
				s.I18N = settings.I18N{Default: "en", Supported: []string{"en", "  "}}
			},
			want: "is empty",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := settings.New(append(prodSettings(), tc.mutate)...)
			if err == nil {
				t.Fatal("expected a configuration error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestTimeZoneErrorMentionsTzdata(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.TimeZone = "Nowhere/Fictional"
	})...)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "time/tzdata") {
		t.Errorf("the message should point at the scratch-image fix: %v", err)
	}
}
