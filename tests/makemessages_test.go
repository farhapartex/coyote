package tests

import (
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/messages"
	"github.com/farhapartex/coyote/core/i18n"
)

func extractGo(t *testing.T, source string) ([]messages.Message, []messages.Problem) {
	t.Helper()
	set, problems, err := messages.FromGo("handlers.go", []byte(source))
	if err != nil {
		t.Fatalf("FromGo: %v", err)
	}
	return set.All(), problems
}

func extractTemplate(t *testing.T, source string) ([]messages.Message, []messages.Problem) {
	t.Helper()
	set, problems, err := messages.FromTemplate("page.html", source)
	if err != nil {
		t.Fatalf("FromTemplate: %v", err)
	}
	return set.All(), problems
}

func singulars(found []messages.Message) []string {
	out := make([]string, 0, len(found))
	for _, message := range found {
		out = append(out, message.Singular)
	}
	return out
}

func TestExtractionFindsGoCalls(t *testing.T) {
	found, problems := extractGo(t, `package main

import "github.com/farhapartex/coyote/core/i18n"

func handler(ctx context.Context, l *i18n.Locale) {
	i18n.T(ctx, "Profile saved.")
	i18n.Tf(ctx, "Welcome back, %s.", name)
	i18n.N(ctx, "%d row updated", "%d rows updated", n)
	i18n.TC(ctx, "verb", "Post")
	l.T("Direct on a locale")
	l.N("%d note", "%d notes", 3)
	l.NC("email", "%d message", "%d messages", 2)
}
`)
	if len(problems) != 0 {
		t.Errorf("unexpected problems: %v", problems)
	}

	got := strings.Join(singulars(found), "|")
	for _, want := range []string{
		"Profile saved.", "Welcome back, %s.", "%d row updated",
		"Post", "Direct on a locale", "%d note", "%d message",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %v", want, singulars(found))
		}
	}

	for _, message := range found {
		switch message.Singular {
		case "%d row updated":
			if message.Plural != "%d rows updated" {
				t.Errorf("plural = %q", message.Plural)
			}
		case "Post":
			if message.Context != "verb" {
				t.Errorf("context = %q", message.Context)
			}
		case "%d message":
			if message.Context != "email" || message.Plural != "%d messages" {
				t.Errorf("context/plural = %q/%q", message.Context, message.Plural)
			}
		}
	}
}

func TestExtractionRecordsReferences(t *testing.T) {
	found, _ := extractGo(t, `package main

func handler() {
	l.T("First")
}
`)
	if len(found) != 1 {
		t.Fatalf("found %d messages", len(found))
	}
	if len(found[0].References) != 1 || !strings.HasPrefix(found[0].References[0], "handlers.go:") {
		t.Errorf("references = %v", found[0].References)
	}
}

func TestExtractionReportsANonConstantMessage(t *testing.T) {
	_, problems := extractGo(t, `package main

func handler() {
	l.T(heading)
	i18n.T(ctx, buildLabel())
}
`)
	if len(problems) != 2 {
		t.Fatalf("problems = %v, want two", problems)
	}
	for _, problem := range problems {
		if !strings.Contains(problem.Reason, "not a literal") {
			t.Errorf("reason = %q", problem.Reason)
		}
		if problem.Line == 0 {
			t.Errorf("problem has no line: %v", problem)
		}
	}
}

func TestExtractionJoinsConcatenatedLiterals(t *testing.T) {
	found, problems := extractGo(t, `package main

func handler() {
	l.T("one " + "two")
}
`)
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	if len(found) != 1 || found[0].Singular != "one two" {
		t.Errorf("found = %v", singulars(found))
	}
}

func TestExtractionSkipsUnrelatedCalls(t *testing.T) {
	found, _ := extractGo(t, `package main

func handler() {
	fmt.Println("not translatable")
	strings.Title("also not")
	other.Nothing("nope")
}
`)
	if len(found) != 0 {
		t.Errorf("found = %v, want nothing", singulars(found))
	}
}

func TestExtractionFindsTemplateCalls(t *testing.T) {
	found, problems := extractTemplate(t, `
{{define "content"}}
  <h1>{{.Locale.T "Your notes"}}</h1>
  <p>{{.Locale.N "%d note" "%d notes" (len .Notes)}}</p>
  <p>{{.Locale.Tf "Hello %s" .Name}}</p>
  {{$t := .Locale}}
  {{range .Notes}}<li>{{$t.T "Saved"}}</li>{{end}}
  {{if .Ready}}{{$.Locale.T "Ready"}}{{else}}{{$.Locale.T "Waiting"}}{{end}}
  {{with .User}}{{$t.TC "noun" "Post"}}{{end}}
  <span>{{.Locale.T "Piped" | printf "%s"}}</span>
{{end}}
`)
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}

	got := strings.Join(singulars(found), "|")
	for _, want := range []string{"Your notes", "%d note", "Hello %s", "Saved", "Ready", "Waiting", "Post", "Piped"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %v", want, singulars(found))
		}
	}
}

func TestExtractionReportsANonConstantTemplateMessage(t *testing.T) {
	_, problems := extractTemplate(t, `{{.Locale.T .Heading}}`)
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want one", problems)
	}
	if !strings.Contains(problems[0].Reason, "not a literal") {
		t.Errorf("reason = %q", problems[0].Reason)
	}
}

func TestExtractionDeduplicatesAcrossFiles(t *testing.T) {
	set := messages.NewSet()
	set.Add(messages.Message{Singular: "Save", References: []string{"a.go:1"}})
	set.Add(messages.Message{Singular: "Save", References: []string{"b.html:4"}})
	set.Add(messages.Message{Singular: "Save", Plural: "Saves", References: []string{"c.go:9"}})

	all := set.All()
	if len(all) != 1 {
		t.Fatalf("found %d entries, want one", len(all))
	}
	if len(all[0].References) != 3 {
		t.Errorf("references = %v", all[0].References)
	}
	if all[0].Plural != "Saves" {
		t.Errorf("a later plural should be adopted, got %q", all[0].Plural)
	}
}

func TestExtractionKeepsContextsSeparate(t *testing.T) {
	set := messages.NewSet()
	set.Add(messages.Message{Singular: "Post"})
	set.Add(messages.Message{Context: "verb", Singular: "Post"})

	if set.Len() != 2 {
		t.Errorf("Len = %d, want the contexts kept apart", set.Len())
	}
}

func TestMergeKeepsExistingTranslations(t *testing.T) {
	existing, err := i18n.ParsePO("fr", strings.NewReader(`msgid ""
msgstr ""
"Language: fr\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

msgid "Keep me"
msgstr "Garde-moi"

msgid "Gone away"
msgstr "Parti"

#, fuzzy
msgid "Unsure"
msgstr "Peut-être"
`))
	if err != nil {
		t.Fatal(err)
	}

	found := []messages.Message{
		{Singular: "Keep me"},
		{Singular: "Unsure"},
		{Singular: "Brand new"},
	}

	document, report := messages.Merge(existing, found, messages.PluralsOf(existing))
	body := document.Render("Test", "fr")

	if !strings.Contains(body, `msgstr "Garde-moi"`) {
		t.Errorf("an existing translation was lost:\n%s", body)
	}
	if !strings.Contains(body, `#~ msgid "Gone away"`) {
		t.Errorf("a removed key should become obsolete, not vanish:\n%s", body)
	}
	if !strings.Contains(body, `#~ msgstr "Parti"`) {
		t.Errorf("an obsolete entry should keep its translation:\n%s", body)
	}
	if !strings.Contains(body, `msgid "Brand new"`+"\n"+`msgstr ""`) {
		t.Errorf("a new key should be added empty:\n%s", body)
	}
	if !strings.Contains(body, "#, fuzzy") {
		t.Errorf("a fuzzy flag should survive:\n%s", body)
	}
	if report.Added != 1 || report.Kept != 2 || report.Obsolete != 1 || report.Fuzzy != 1 {
		t.Errorf("report = %+v", report)
	}
}

func TestMergeKeepsTheExistingHeader(t *testing.T) {
	existing, err := i18n.ParsePO("ar", strings.NewReader(`msgid ""
msgstr ""
"Language: ar\n"
"Plural-Forms: nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 ? 4 : 5);\n"
"Last-Translator: Someone\n"
`))
	if err != nil {
		t.Fatal(err)
	}

	document, _ := messages.Merge(existing, []messages.Message{
		{Singular: "%d note", Plural: "%d notes"},
	}, messages.PluralsOf(existing))
	body := document.Render("Test", "ar")

	if !strings.Contains(body, "Last-Translator: Someone") {
		t.Errorf("the header should be preserved:\n%s", body)
	}
	if !strings.Contains(body, "nplurals=6") {
		t.Errorf("the plural rule should be preserved:\n%s", body)
	}
	if !strings.Contains(body, "msgstr[5]") {
		t.Errorf("a plural entry should get six forms in Arabic:\n%s", body)
	}
}

func TestMergeIntoAnEmptyCatalogWritesAHeader(t *testing.T) {
	document, report := messages.Merge(nil, []messages.Message{{Singular: "First"}}, 2)
	body := document.Render("My project", "fr")

	if !strings.Contains(body, "Language: fr") {
		t.Errorf("a new catalog needs a header:\n%s", body)
	}
	if !strings.Contains(body, "My project") {
		t.Errorf("the project name should appear:\n%s", body)
	}
	if report.Added != 1 || report.Kept != 0 {
		t.Errorf("report = %+v", report)
	}
}

func TestMergeOutputParsesBackIn(t *testing.T) {
	document, _ := messages.Merge(nil, []messages.Message{
		{Singular: `He said "hi"`},
		{Singular: "line\nbreak"},
		{Context: "verb", Singular: "Post"},
		{Singular: "%d note", Plural: "%d notes"},
	}, 2)
	body := document.Render("Test", "fr")

	reparsed, err := i18n.ParsePO("fr", strings.NewReader(body))
	if err != nil {
		t.Fatalf("the written catalog does not parse: %v\n%s", err, body)
	}

	entries := i18n.Entries(reparsed)
	if len(entries) < 4 {
		t.Errorf("round trip lost entries: %d\n%s", len(entries), body)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Singular] = true
	}
	for _, want := range []string{`He said "hi"`, "line\nbreak", "Post", "%d note"} {
		if !seen[want] {
			t.Errorf("round trip lost %q:\n%s", want, body)
		}
	}
}

func TestTemplateRenderIsValidPO(t *testing.T) {
	body := messages.RenderTemplate("Test", []messages.Message{
		{Singular: "Save", References: []string{"page.html:3"}},
		{Singular: "%d note", Plural: "%d notes", References: []string{"page.html:4"}},
	})

	if !strings.Contains(body, "#: page.html:3") {
		t.Errorf("references should be written:\n%s", body)
	}
	if _, err := i18n.ParsePO("en", strings.NewReader(body)); err != nil {
		t.Errorf("the template does not parse: %v\n%s", err, body)
	}
}
