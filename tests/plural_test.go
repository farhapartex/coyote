package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/i18n"
)

func TestPluralRulesMatchThePublishedForms(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		forms  int
		want   map[int]int
	}{
		"english": {
			header: "nplurals=2; plural=(n != 1);",
			forms:  2,
			want:   map[int]int{0: 1, 1: 0, 2: 1, 21: 1, 100: 1},
		},
		"french": {
			header: "nplurals=2; plural=(n > 1);",
			forms:  2,
			want:   map[int]int{0: 0, 1: 0, 2: 1, 100: 1},
		},
		"japanese": {
			header: "nplurals=1; plural=0;",
			forms:  1,
			want:   map[int]int{0: 0, 1: 0, 2: 0, 999: 0},
		},
		"russian": {
			header: "nplurals=3; plural=(n%10==1 && n%100!=11 ? 0 : n%10>=2 && n%10<=4 && (n%100<10 || n%100>=20) ? 1 : 2);",
			forms:  3,
			want: map[int]int{
				1: 0, 21: 0, 101: 0,
				2: 1, 3: 1, 4: 1, 22: 1,
				5: 2, 11: 2, 12: 2, 14: 2, 25: 2, 100: 2, 111: 2,
			},
		},
		"polish": {
			header: "nplurals=3; plural=(n==1 ? 0 : n%10>=2 && n%10<=4 && (n%100<10 || n%100>=20) ? 1 : 2);",
			forms:  3,
			want:   map[int]int{1: 0, 2: 1, 3: 1, 4: 1, 5: 2, 22: 1, 25: 2, 112: 2},
		},
		"arabic": {
			header: "nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 ? 4 : 5);",
			forms:  6,
			want:   map[int]int{0: 0, 1: 1, 2: 2, 3: 3, 10: 3, 11: 4, 99: 4, 100: 5, 102: 5, 103: 3},
		},
		"welsh": {
			header: "nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n==3 ? 3 : n==6 ? 4 : 5);",
			forms:  6,
			want:   map[int]int{0: 0, 1: 1, 2: 2, 3: 3, 6: 4, 7: 5, 42: 5},
		},
		"czech": {
			header: "nplurals=3; plural=(n==1) ? 0 : (n>=2 && n<=4) ? 1 : 2;",
			forms:  3,
			want:   map[int]int{1: 0, 2: 1, 4: 1, 5: 2, 0: 2},
		},
	} {
		t.Run(name, func(t *testing.T) {
			rule, err := i18n.ParsePluralForms(tc.header)
			if err != nil {
				t.Fatalf("ParsePluralForms: %v", err)
			}
			if rule.Forms() != tc.forms {
				t.Errorf("Forms = %d, want %d", rule.Forms(), tc.forms)
			}
			for n, want := range tc.want {
				if got := rule.Form(n); got != want {
					t.Errorf("Form(%d) = %d, want %d", n, got, want)
				}
			}
		})
	}
}

func TestPluralRuleWithoutAHeaderFallsBackToEnglish(t *testing.T) {
	rule, err := i18n.ParsePluralForms("")
	if err != nil {
		t.Fatalf("ParsePluralForms: %v", err)
	}
	if rule.Form(1) != 0 || rule.Form(2) != 1 || rule.Form(0) != 1 {
		t.Errorf("an absent rule should behave like n != 1")
	}
}

func TestPluralRuleRejectsNonsenseButStaysUsable(t *testing.T) {
	for _, header := range []string{
		"nplurals=2; plural=(n !! 1);",
		"nplurals=2; plural=(n > );",
		"nplurals=2; plural=n ? 1;",
		"nplurals=0; plural=0;",
		"nplurals=2; plural=drop table users;",
		"nplurals=2; plural=((n);",
	} {
		rule, err := i18n.ParsePluralForms(header)
		if err == nil {
			t.Errorf("%q should not parse", header)
		}
		if !errors.Is(err, i18n.ErrBadPluralRule) {
			t.Errorf("%q gave %v, want ErrBadPluralRule", header, err)
		}
		if got := rule.Form(2); got < 0 || got >= rule.Forms() {
			t.Errorf("%q left an unusable rule: Form(2) = %d of %d", header, got, rule.Forms())
		}
	}
}

func TestPluralRuleSurvivesDivisionByZero(t *testing.T) {
	rule, err := i18n.ParsePluralForms("nplurals=2; plural=(n%0 == 1);")
	if err != nil {
		t.Fatalf("ParsePluralForms: %v", err)
	}
	if got := rule.Form(7); got < 0 || got > 1 {
		t.Errorf("Form = %d, want a usable form rather than a panic", got)
	}
}

func TestPluralRuleClampsAFormOutOfRange(t *testing.T) {
	rule, err := i18n.ParsePluralForms("nplurals=2; plural=5;")
	if err != nil {
		t.Fatalf("ParsePluralForms: %v", err)
	}
	if got := rule.Form(1); got != 0 {
		t.Errorf("Form = %d, want it clamped into range", got)
	}
}

func TestPluralRuleKeepsItsSource(t *testing.T) {
	header := "nplurals=2; plural=(n != 1);"
	rule, err := i18n.ParsePluralForms(header)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rule.Source(), "nplurals=2") {
		t.Errorf("Source = %q", rule.Source())
	}
}
