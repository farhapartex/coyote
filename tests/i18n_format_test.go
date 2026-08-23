package tests

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/settings"
)

func formattingBundle(t *testing.T, zone string) *i18n.Bundle {
	t.Helper()
	location := time.UTC
	if zone != "" {
		loaded, err := time.LoadLocation(zone)
		if err != nil {
			t.Skipf("zone %s unavailable: %v", zone, err)
		}
		location = loaded
	}
	bundle := i18n.NewBundle(i18n.Options{
		Default:   "en",
		Supported: []string{"en", "en-US", "fr", "de", "hi", "ja", "ar"},
		Zone:      location,
	})
	if problems := i18n.LoadInto(bundle, i18n.NewFSLoader(fstest.MapFS{
		"fr.po": &fstest.MapFile{Data: []byte(monthsFrenchPO)},
	}, "")); len(problems) > 0 {
		t.Logf("loader: %v", problems)
	}
	return bundle
}

const monthsFrenchPO = `msgid ""
msgstr ""
"Language: fr\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

msgid "March"
msgstr "mars"
`

func TestNumberGroupingPerLocale(t *testing.T) {
	bundle := formattingBundle(t, "")

	for tag, want := range map[string]string{
		"en": "1,234,567.89",
		"fr": "1\u00a0234\u00a0567,89",
		"de": "1.234.567,89",
		"ja": "1,234,567.89",
	} {
		if got := bundle.Locale(tag).Number(1234567.89); got != want {
			t.Errorf("Number in %s = %q, want %q", tag, got, want)
		}
	}
}

func TestNumberUsesIndianGrouping(t *testing.T) {
	bundle := formattingBundle(t, "")

	if got := bundle.Locale("hi").Number(12345678); got != "1,23,45,678" {
		t.Errorf("Hindi grouping = %q, want lakh and crore grouping", got)
	}
	if got := bundle.Locale("en").Number(12345678); got != "12,345,678" {
		t.Errorf("English grouping = %q", got)
	}
}

func TestNumberHandlesEveryNumericKind(t *testing.T) {
	locale := formattingBundle(t, "").Locale("en")

	for _, value := range []any{1000, int64(1000), uint(1000), float32(1000), 1000.0, "1000"} {
		if got := locale.Number(value); got != "1,000" {
			t.Errorf("Number(%T) = %q, want 1,000", value, got)
		}
	}
	if got := locale.Number(-1234.5); got != "-1,234.5" {
		t.Errorf("a negative number = %q", got)
	}
	if got := locale.Number(12); got != "12" {
		t.Errorf("a short number should not be grouped, got %q", got)
	}
	if got := locale.Number("not a number"); got != "not a number" {
		t.Errorf("an unparseable value should pass through, got %q", got)
	}
}

func TestMoneyPlacesTheSymbolPerLocale(t *testing.T) {
	bundle := formattingBundle(t, "")

	for tag, want := range map[string]string{
		"en": "$1,234.50",
		"fr": "1\u00a0234,50\u00a0€",
		"de": "1.234,50\u00a0€",
	} {
		currency := "USD"
		if tag != "en" {
			currency = "EUR"
		}
		if got := bundle.Locale(tag).Money(1234.5, currency); got != want {
			t.Errorf("Money in %s = %q, want %q", tag, got, want)
		}
	}
}

func TestMoneyRespectsCurrencyDigits(t *testing.T) {
	locale := formattingBundle(t, "").Locale("en")

	if got := locale.Money(1234, "JPY"); got != "¥1,234" {
		t.Errorf("yen has no minor unit, got %q", got)
	}
	if got := locale.Money(1234, "USD"); got != "$1,234.00" {
		t.Errorf("Money = %q", got)
	}
	if got := locale.Money(1234.5, "XYZ"); got != "XYZ1,234.50" {
		t.Errorf("an unknown currency should use its code, got %q", got)
	}
}

func TestDateAndTimeOrderPerLocale(t *testing.T) {
	bundle := formattingBundle(t, "")
	when := time.Date(2026, time.March, 4, 15, 30, 0, 0, time.UTC)

	for tag, want := range map[string]string{
		"en":    "04/03/2026",
		"en-US": "03/04/2026",
		"de":    "04.03.2026",
		"ja":    "2026/03/04",
	} {
		if got := bundle.Locale(tag).Date(when); got != want {
			t.Errorf("Date in %s = %q, want %q", tag, got, want)
		}
	}

	if got := bundle.Locale("en").Time(when); got != "15:30" {
		t.Errorf("24 hour time = %q", got)
	}
	if got := bundle.Locale("en-US").Time(when); got != "3:30 PM" {
		t.Errorf("12 hour time = %q", got)
	}
}

func TestLongDateUsesTranslatedMonthNames(t *testing.T) {
	bundle := formattingBundle(t, "")
	when := time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC)

	if got := bundle.Locale("en").LongDate(when); got != "4 March 2026" {
		t.Errorf("English long date = %q", got)
	}
	if got := bundle.Locale("fr").LongDate(when); got != "4 mars 2026" {
		t.Errorf("French long date = %q; month names come from the catalog", got)
	}
	if got := bundle.Locale("en-US").LongDate(when); got != "March 4, 2026" {
		t.Errorf("American long date = %q", got)
	}
}

func TestTimestampsRenderInTheConfiguredZone(t *testing.T) {
	bundle := formattingBundle(t, "Asia/Tokyo")
	when := time.Date(2026, time.March, 4, 23, 30, 0, 0, time.UTC)

	locale := bundle.Locale("en")
	if got := locale.Zone().String(); got != "Asia/Tokyo" {
		t.Fatalf("Zone = %q", got)
	}
	if got := locale.Time(when); got != "08:30" {
		t.Errorf("Time = %q, want the Tokyo wall clock", got)
	}
	if got := locale.Date(when); got != "05/03/2026" {
		t.Errorf("Date = %q, want the following day in Tokyo", got)
	}
}

func TestUTCIsTheDefaultZone(t *testing.T) {
	bundle := formattingBundle(t, "")
	when := time.Date(2026, time.March, 4, 23, 30, 0, 0, time.UTC)

	if got := bundle.Locale("en").Time(when); got != "23:30" {
		t.Errorf("Time = %q, want UTC", got)
	}
}

func TestNilLocaleStillFormats(t *testing.T) {
	var locale *i18n.Locale
	when := time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC)

	if got := locale.Number(1234); got != "1,234" {
		t.Errorf("Number = %q", got)
	}
	if got := locale.Date(when); got != "04/03/2026" {
		t.Errorf("Date = %q", got)
	}
	if locale.Zone() != time.UTC {
		t.Errorf("Zone = %v, want UTC", locale.Zone())
	}
}

func TestAnUnknownLocaleFallsBackToTheDefaultPatterns(t *testing.T) {
	bundle := i18n.NewBundle(i18n.Options{Default: "en", Supported: []string{"en", "xx"}})

	if got := bundle.Locale("xx").Number(1234.5); got != "1,234.5" {
		t.Errorf("Number = %q, want the default pattern", got)
	}
}

func TestCustomFormatterReplacesTheBuiltIn(t *testing.T) {
	bundle := i18n.NewBundle(i18n.Options{
		Default:   "en",
		Supported: []string{"en"},
		Formatter: stubFormatter{},
	})

	locale := bundle.Locale("en")
	if got := locale.Number(1234); got != "stub-number" {
		t.Errorf("Number = %q, want the supplied formatter", got)
	}
	if got := locale.Money(1234, "USD"); got != "stub-money" {
		t.Errorf("Money = %q", got)
	}
	if got := locale.LongDate(time.Now()); got != "stub-date" {
		t.Errorf("a formatter without LongDate should fall back to Date, got %q", got)
	}
}

type stubFormatter struct{}

func (stubFormatter) Date(time.Time) string     { return "stub-date" }
func (stubFormatter) Time(time.Time) string     { return "stub-time" }
func (stubFormatter) DateTime(time.Time) string { return "stub-datetime" }
func (stubFormatter) Number(any) string         { return "stub-number" }
func (stubFormatter) Money(any, string) string  { return "stub-money" }

func TestSettingsAcceptACustomFormatter(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.I18N = settings.I18N{
			Default:   "en",
			Supported: []string{"en"},
			Formatter: stubFormatter{},
		}
	})

	if got := a.Bundle().Locale("en").Number(99); got != "stub-number" {
		t.Errorf("Number = %q", got)
	}
}

func TestAnUnloadableZoneDegradesToUTC(t *testing.T) {
	bundle := i18n.NewBundle(i18n.Options{Default: "en", Supported: []string{"en"}})
	if bundle.Locale("en").Zone() != time.UTC {
		t.Error("a bundle with no zone should use UTC")
	}
}
