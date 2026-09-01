# Forms and validation

[← Back to contents](README.md)

Two ways in: bind a request into **your struct**, or bind it against a **model schema**. Both live
in `core/form` and report problems the same way.

## Binding a struct

```go
type Signup struct {
	Email    string `form:"email" validate:"required,email"`
	Name     string `form:"name" validate:"required,max=60"`
	Age      int    `form:"age" validate:"min=18"`
	Accepted bool   `form:"accepted" validate:"required"`
}

func signup(w http.ResponseWriter, r *http.Request) {
	var in Signup
	problems, err := form.Bind(r, &in)
	if err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}
	if problems.Any() {
		a.RenderStatus(w, r, 422, "pages/signup.html", app.Data{"Problems": problems, "Form": in})
		return
	}
	...
}
```

`form:"name"` names the field (the lowercased Go name by default; `-` skips it). Values are trimmed,
and `form.BindQuery` does the same for the query string.

Supported field types: `string`, every `int`/`uint`/`float` width, `bool` (`1`, `true`, `on`, `yes`,
`checked`), `time.Time`, and pointers to any of them — a pointer stays nil when the field is absent,
which is how you tell "empty" from "not sent".

**A number is parsed at its own width.** `200` into an `int8` is `is out of range`, not `-56`. That
matters beyond tidiness: `validate` rules read the submitted text, so a value that had silently
wrapped would have been checked as the number the visitor typed and stored as a different one.

## Problems

```go
type Problems map[string][]string
```

Every field is checked, so the visitor sees **all** the mistakes at once rather than one per
submission.

```go
problems.Any()          // anything wrong?
problems.Has("email")   // this field?
problems.First("email") // the first message
problems.Fields()       // sorted field names
```

```html
{{with .Problems}}
  {{range $field, $messages := .}}
    {{range $messages}}<p class="error">{{$field}} {{.}}</p>{{end}}
  {{end}}
{{end}}
```

## Built-in rules

| Rule | Rejects |
| --- | --- |
| `required` | empty or whitespace |
| `email` | anything `net/mail` will not parse |
| `url` | no scheme or no host |
| `min=N` | numbers below N, or strings shorter than N |
| `max=N` | numbers above N, or strings longer than N |
| `len=N` | strings that are not exactly N runes |
| `oneof=a\|b\|c` | anything outside the list |
| `match=regexp` | anything the pattern does not match |
| `numeric` | anything that is not a number |
| `alphanum` | anything but letters and digits |

Only `required` objects to an empty value — the rest pass it, so optional fields validate only when
filled in.

## Your own rules

```go
form.Register("shouty", func(value, arg string) error {
	if value != strings.ToUpper(value) {
		return errors.New("must be shouty")
	}
	return nil
})
```

```go
Word string `form:"word" validate:"shouty"`
```

## Binding against a model

When the shape comes from a model rather than a struct — which is how the admin and the
[generic views](07-views.md) work:

```go
schema, _ := a.Describe(Product{})
bound := form.Record(r.PostForm, schema, true)   // true = creating

if !bound.Valid() {
	// bound.Problems, keyed by column
}
store.Insert(r.Context(), schema, bound.Record)
```

Columns are converted by their `model.Kind`, required columns are enforced, and `size` is checked.
The admin portal uses exactly this — there is one binder, not two.

## Next

- [Views →](07-views.md) — rendering the result
- [Uploads →](31-uploads.md) — files arriving with the form
