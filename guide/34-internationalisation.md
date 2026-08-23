# Internationalisation

[← Back to contents](README.md)

Translations come from gettext PO files, so your translators can use the tools they already have. A
project that never mentions locales pays nothing: one locale, no catalogs loaded, and every
`{{.Locale.T "…"}}` returns its own English.

## Configuring it

```go
settings.Configure(func(s *settings.Settings) {
	s.I18N = settings.I18N{
		Default:   "en",
		Supported: []string{"en", "fr", "ar"},
		Dir:       "locales",
		Fallbacks: map[string]string{"fr-CA": "fr"},
	}
	s.TimeZone = "Europe/Paris"
})
```

```
locales/
  fr.po
  ar.po
```

English needs no file — an untranslated message falls back to the text in your template, which is
already English. Embed the catalogs instead of reading them from disk with `I18N.FS`:

```go
//go:embed locales
var localeFS embed.FS

sub, _ := fs.Sub(localeFS, "locales")
s.I18N.FS = sub
```

A catalog that cannot be read is a **warning, not a fatal error** — the site runs in English rather
than refusing to start. That is the opposite of the [cache](33-caching.md) decision, and deliberately
so: a missing translation has a sane fallback, a missing Redis does not.

`TimeZone` is validated at startup and **fails if the zone cannot be loaded**, because a mistyped zone
silently falling back to UTC is how a year of reports comes out wrong. On a `scratch` image there is no
zoneinfo, so add `import _ "time/tzdata"` to your main package.

## Translating a template

```html
{{define "content"}}
  <h1>{{.Locale.T "Your notes"}}</h1>
  <p>{{.Locale.N "%d note" "%d notes" (len .Notes)}}</p>
  <p>{{.Locale.Tf "Welcome back, %s." .User.Username}}</p>
{{end}}
```

| Method | For |
| --- | --- |
| `.T "text"` | a plain string |
| `.Tf "text %s" x` | a string with placeholders |
| `.N "one" "many" n` | a count, using the locale's own plural rules |
| `.TC "context" "text"` | the same English word meaning two things |
| `.NC "context" …` | both at once |

**The message id is the English source string**, not a symbolic key. So a template reads as English
before any translation exists, and a missing translation renders that same English rather than
`admin.user.form.save`.

### Inside a range, use `$`

`{{range}}` rebinds the dot, so `.Locale` disappears. Bind it once at the top instead:

```html
{{$t := .Locale}}
{{range .Notes}}
  <li>{{$t.T "Saved"}} — {{.Title}}</li>
{{end}}
```

`{{$.Locale.T "Saved"}}` works too. This is the one wart in the API, and it is the same `$` you
already use for any outer value inside a loop.

### Why not `{{t "…"}}`

Because `html/template` cannot do it safely. A `FuncMap` is bound when the template is parsed, so a
package-level `t` cannot know which request is rendering — the locale would be ambient state shared
between concurrent requests. `Clone` per request is the usual workaround, but it errors once a template
has executed and re-runs the escaping analysis, which throws away the template cache.

Putting the function in the render data does not work either: `html/template` answers *"T is not a
method but has arguments"* for a function stored in a map. Only a method on a value can take
arguments, so the locale is an object.

## Translating from Go

```go
view.Success(r, i18n.T(r.Context(), "Profile saved."))
view.Error(r, i18n.Tf(r.Context(), "Could not save %s.", name))

message := i18n.N(r.Context(), "%d row updated", "%d rows updated", n)
```

Symmetrical with the templates: both read the locale from where they are, neither uses ambient state.
A context with no locale still translates to the source string, so a handler under test does not need
the middleware.

`i18n.Tf` is checked by `go vet` like any printf wrapper, so a `%s`/`%d` mismatch between your source
string and a translation is caught at build time. `i18n.T` takes no arguments and accepts a computed
msgid — though a computed msgid cannot be extracted, so prefer constants.

## Catalogs

A PO file is plain text with comments, which is why translator tooling exists for it:

```po
msgid ""
msgstr ""
"Language: fr\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

#: templates/pages/notes.html:4
msgid "Your notes"
msgstr "Vos notes"

msgid "%d note"
msgid_plural "%d notes"
msgstr[0] "%d note"
msgstr[1] "%d notes"

msgctxt "verb"
msgid "Post"
msgstr "Publier"
```

The parser handles multi-line strings, `\n` and `\"` escapes, `msgctxt`, CRLF and a byte-order mark. Two
rules that follow gettext rather than surprising you:

- **An empty `msgstr` means untranslated**, so it falls back rather than rendering nothing.
- **A `#, fuzzy` entry is not used.** Fuzzy means "a human has not confirmed this", and shipping it is
  worse than shipping English.
- An obsolete `#~` entry is ignored.

## Plural rules come from the file, not from us

Arabic has six plural categories, Russian four, Japanese one. Rather than bundle a CLDR table, each PO
file declares its own rule in its header:

```
"Plural-Forms: nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 ? 4 : 5);\n"
```

Coyote evaluates that expression. **Adding a language therefore needs no change to the framework** — the
translator's file carries its own grammar. The published rules for English, French, Japanese, Russian,
Polish, Czech, Welsh and Arabic are all covered by tests.

A missing header falls back to `n != 1`. A malformed one logs and falls back rather than breaking a
render, and a rule that divides by zero cannot crash a page.

## Which locale a request gets

In order, first match wins:

| | Source |
| --- | --- |
| 1 | a URL prefix, when `I18N.URLPrefix` is on |
| 2 | the `coyote_locale` cookie |
| 3 | the `Accept-Language` header, honouring q-values |
| 4 | `I18N.Default` |

It always resolves to something. An unsupported tag from any source is ignored rather than trusted, so
a hand-edited cookie cannot reach a locale you do not serve.

Multilingual sites get `Vary: Accept-Language` automatically, which is what lets a
[cached page](33-caching.md) hold one entry per language instead of serving the first visitor's
language to everyone.

**The session is not consulted.** The item list originally put it second, and it was dropped for a
concrete reason: the page cache must sit outside the session middleware to see a `Set-Cookie` and
refuse to cache a personalised response, and locale detection must sit outside the page cache so the
cache key can include the locale. Detection therefore runs outside the session and cannot read it. The
cookie covers the same need — it lasts a year.

### URL prefixes

```go
s.I18N.URLPrefix = true
```

```
/about        English, the default locale
/fr/about     French
/en/about     301 → /about
```

The default locale has **no prefix**, and an explicit default prefix redirects to the canonical
unprefixed path with the query string intact. That means a site which was monolingual keeps every
existing link and bookmark working when it adds a language, and each page still has exactly one
canonical URL.

The prefix is stripped before routing, so one route serves every locale:

```go
a.Get("/about", about)     // serves /about and /fr/about
```

Build prefixed links with `Path`, which leaves the default locale alone:

```html
<a href="{{.Locale.Path (url "about")}}">{{.Locale.T "About"}}</a>
```

### Letting the visitor choose

```go
a.Post("/locale", a.Locales().SwitchHandler("/"), a.CSRF)
```

```html
<form method="post" action="/locale">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  <input type="hidden" name="next" value="{{.Path}}">
  {{$active := .Locale.Tag}}
  {{range .Locale.Available}}
    <button name="locale" value="{{.Tag}}" {{if .Active}}disabled{{end}}>{{.Name}}</button>
  {{end}}
</form>
```

`Available` gives you the tag, the language's own name for itself, its direction, and which one is
active. `SwitchHandler` writes the cookie, refuses a locale you do not support, and returns the visitor
to `next` — checked to be a same-origin path, so it cannot be turned into an open redirect. With URL
prefixes on it also moves them to the same page under the new locale.

## Missing translations

Resolution runs **active locale → fallback chain → the source string**. It never renders an empty
string.

```go
s.I18N.Fallbacks = map[string]string{"fr-CA": "fr"}
```

`fr-CA` then tries `fr-CA`, `fr`, and finally the default, so a regional catalog only needs the strings
that actually differ.

Under `Debug` a missing key is logged **once per key**, not once per render — a template inside a
`range` would otherwise write ten thousand identical warnings. `a.Bundle()` and `Locale.Missing()`
report what was missed, which is enough to put a list on an internal page.

## Dates, numbers and money

```html
<p>{{.Locale.Date .CreatedAt}}</p>        <!-- 04/03/2026 -->
<p>{{.Locale.LongDate .CreatedAt}}</p>    <!-- 4 March 2026 -->
<p>{{.Locale.Time .CreatedAt}}</p>        <!-- 15:30, or 3:30 PM -->
<p>{{.Locale.DateTime .CreatedAt}}</p>
<p>{{.Locale.Number .Total}}</p>          <!-- 1,234,567.89 -->
<p>{{.Locale.Money .Price "EUR"}}</p>     <!-- 1 234,50 € in French -->
```

What the built-in formatter knows, per locale: the date field order, 12- or 24-hour time, the decimal
and grouping separators, digit grouping (including Indian lakh and crore, so `hi` gives `1,23,45,678`),
and whether the currency symbol leads or trails and whether a no-break space sits between. Currency
minor units come from the currency, not the locale — yen and won get no decimals, Kuwaiti dinar gets
three.

`LongDate` is the interesting one: **month names come from your catalog.** The formatter asks for
`January` … `December` through the same lookup as everything else, so a translator supplies them once
and every long date in the project is right. There is no month-name table in the framework.

### It is not CLDR, and here is the honest boundary

The table covers about twenty common locales and falls back to a sane default. It gets the separators,
the field order and the symbol placement right for those; it does **not** implement the full CLDR
pattern algebra, alternate calendars, or accounting negatives. That is a deliberate trade: full CLDR
means linking `golang.org/x/text`'s tables into every binary, including the single-locale ones.

If you need it, the seam is one line:

```go
type Formatter interface {
	Date(t time.Time) string
	Time(t time.Time) string
	DateTime(t time.Time) string
	Number(value any) string
	Money(value any, currency string) string
}
```

```go
s.I18N.Formatter = myTextFormatter{}   // wrapping golang.org/x/text/message
```

Implement `LongDate(time.Time) string` too and it is used; leave it out and `Date` is used instead.

## Time zones

```go
s.TimeZone = "Europe/Paris"
```

`.Date`, `.Time`, `.DateTime` and `.LongDate` all render in that zone. The database keeps UTC — this is
a presentation setting, not a storage one. `.Locale.Zone()` gives you the `*time.Location` if you need
it in a handler.

The zone is loaded and validated at startup, and a bad value is a **configuration error rather than a
silent fall back to UTC**, because a mistyped zone quietly shifting every timestamp is the kind of bug
that is found a year later. On a `scratch` image there is no system zoneinfo, so add
`import _ "time/tzdata"` to your main package.

## Extracting the messages

```
coyote makemessages
coyote makemessages --locale=fr
```

It walks your project, reads **Go source with `go/ast`** and **templates with the standard template
parser** — not regular expressions, so `{{.Locale.T "x" | printf "%s"}}` and calls nested inside
`{{range}}` or `{{if}}` are all found. Then it writes `locales/coyote.pot` and merges into every
catalog:

```
$ coyote makemessages
scanned    2 file(s)
extracted  4 message(s)
catalogs   /srv/app/locales

wrote coyote.pot
merged fr.po        4 new, 0 kept, 0 fuzzy, 0 obsolete
```

**The merge never loses a translator's work.** Existing translations are kept, fuzzy flags survive, new
keys arrive empty, and a message that has disappeared from the source is **commented out as `#~` with
its translation intact** rather than deleted. If it comes back next week, the translation is still
there.

`_test.go` files are skipped, as are `vendor`, `node_modules`, `migrations` and the catalog directory
itself. Plural entries get as many `msgstr[n]` slots as the catalog's own `Plural-Forms` header
declares, so an Arabic file gets six.

A message the extractor cannot read is **reported, not silently dropped**:

```
1 message(s) could not be extracted:
  ! handlers.go:12: T was called with a value that is not a literal, so it cannot be extracted
```

That is the trade for using the source string as the key: `l.T(heading)` works at runtime but no tool
can find it, so the extractor tells you rather than letting a string quietly go untranslated.

### Month names

`LongDate` looks up `January` … `December` through the catalog, and those lookups are computed rather
than literal, so the extractor cannot see them. Add the twelve names to your catalog by hand once, or
copy them from the framework's own `.pot`.

## Checking the catalogs in CI

```
coyote checkmessages
coyote checkmessages --locale=fr --strict
```

```
$ coyote checkmessages
extracted  4 message(s) from 2 file(s)
catalogs   /srv/app/locales

fr       25% translated (1 of 4)
  3 untranslated
    - %d note
    - Item
    - Profile saved.
```

It **exits non-zero** when anything is untranslated or a catalog is missing, so it drops straight into a
pipeline. `--strict` also fails on fuzzy entries, obsolete entries, and messages that could not be
extracted — which is what you want on a release branch rather than on every commit.

## The admin portal is translated

Every string in the admin portal and in `contrib/accounts` goes through the same translation function
your own pages use, and both ship an embedded catalog. Mount the portal in a project with `fr` in
`I18N.Supported` and it is in French, with no catalog of your own:

```
Tableau de bord · Utilisateurs · Rôles · Sessions · Déconnexion
```

French ships today. Other locales are a matter of translating the `.pot` that ships beside the
templates — `contrib/admin/locales/coyote.pot` and `contrib/accounts/locales/coyote.pot`, both generated
by `makemessages` from the framework's own source.

### Overriding a framework string

Your catalog is consulted **before** the framework's, so changing one word needs one entry rather than
a fork:

```po
msgid "Dashboard"
msgstr "Mon tableau"
```

Everything you do not override still comes from the framework's catalog.

### A locale with no catalog

The portal falls back to English, string by string — never to blanks. An Arabic visitor with no Arabic
catalog gets English text in a right-to-left layout, which is ugly but usable, and is what you want
while a translation is in progress.

## Right to left

```html
<html lang="{{.Locale.Tag}}" dir="{{.Locale.Direction}}">
```

`Direction()` returns `ltr` or `rtl`, and `RTL()` is the boolean form. Arabic, Hebrew, Persian, Urdu,
Pashto and Divehi are recognised.

**The admin portal is RTL-correct.** Its stylesheet uses no directional properties at all — flexbox and
logical properties only — and a test fails the build if `margin-left`, `text-align: left` or any of
their siblings reappears. That is cheaper to keep than to retrofit.

Styling your own pages is your business; use logical properties (`margin-inline-start` rather than
`margin-left`) and flexbox, and most layouts mirror themselves for nothing.

## Every setting

| Setting | Default | Notes |
| --- | --- | --- |
| `I18N.Default` | `en` | Must appear in `Supported` if that is set |
| `I18N.Supported` | `["en"]` | More than one turns detection and `Vary` on |
| `I18N.Dir` | `locales` | Resolved against `BaseDir`; ignored when `FS` is set |
| `I18N.FS` | none | An `fs.FS`, usually from `go:embed` |
| `I18N.Fallbacks` | none | `{"fr-CA": "fr"}`; regional to base is automatic |
| `I18N.URLPrefix` | `false` | `/fr/about`, with the default locale unprefixed |
| `I18N.CookieName` | `coyote_locale` | |
| `I18N.Loader` | PO from `FS` or `Dir` | Any `i18n.Loader`, for catalogs from elsewhere |
| `I18N.Formatter` | built-in table | Any `i18n.Formatter`, for full CLDR via `x/text` |
| `TimeZone` | `UTC` | Validated at startup; needs `time/tzdata` on a scratch image |

## Your own catalog source

```go
type Loader interface {
	Load(tag string) (Catalog, error)
}
```

```go
type Catalog interface {
	Tag() string
	Lookup(context, msgid string) (string, bool)
	LookupPlural(context, msgid string, n int) (string, bool)
	Plurals() int
	Header(name string) string
}
```

Implement `Loader` to read translations from a database, an API, or JSON, and set `I18N.Loader`. A
`false` second return means "not translated here", which is what makes the fallback chain work — do not
return an empty string instead.

## Not here yet

Deliberately **not** planned: `core/form` validation messages are not translated. `form.Problems` is
`map[string][]string` and its rules return finished English sentences, so a French visitor submitting a
bad form sees English. Fixing it properly means `Problems` carrying a rule name and arguments and
formatting at the presentation layer — a change to a public type that belongs with a validation pass
rather than here.

## Next

[Caching →](33-caching.md)
