# Pagination

[← Back to contents](README.md)

Ten items a page, everywhere, until you say otherwise.

```go
s.Pagination.PerPage = 25
```

Resolution runs **per-view override → your setting → the framework default of 10**, so a value you
set is always preferred and you never have to restate the default.

## Turning it off

```go
s.Pagination.PerPage = 0
```

Zero means no limit and no controls: every list renders in full and the pager disappears from the
page. That is deliberate rather than using a nil paginator, because nil is also the value of a field
nobody set — which would silently disable paging for projects that never mention it.

## The Page value

Every paged view puts a `Page` in its template data:

```go
type Page struct {
	Enabled bool
	Number  int
	PerPage int
	Total   int64
	Pages   int
	Offset  int
	Limit   int
	HasPrev bool
	HasNext bool
	Prev    int
	Next    int
	First   int
	Last    int
	Numbers []int
}
```

`Offset` and `Limit` feed straight into a `model.Query`. `Numbers` is a window around the current
page, ready to render as links. `Empty()` and `Single()` answer the two questions a template usually
asks.

```html
{{with .Page}}
{{if and .Enabled (not .Single)}}
<nav class="pager">
  {{if .HasPrev}}<a href="?page={{.Prev}}">Previous</a>{{end}}
  {{range .Numbers}}<a href="?page={{.}}">{{.}}</a>{{end}}
  {{if .HasNext}}<a href="?page={{.Next}}">Next</a>{{end}}
</nav>
<p>Page {{.Number}} of {{.Pages}} · {{.Total}} total</p>
{{end}}
{{end}}
```

## Paging by hand

```go
page := view.Paginate(nil, total, number, perPage)

result, err := store.List(r.Context(), schema, model.Query{
	Limit:  page.Limit,
	Offset: page.Offset,
})
```

A page number past the end is clamped to the last page, and anything below 1 becomes 1 — so a
hand-edited URL cannot produce an empty screen or a negative offset.

## Your own paginator

Django's `pagination_class` has no direct equivalent in Go, because Go has no classes. The
equivalent is an **interface**:

```go
type Paginator interface {
	Paginate(total int64, number, perPage int) Page
}
```

```go
type wideWindow struct{}

func (wideWindow) Paginate(total int64, number, perPage int) view.Page {
	return view.Pages{Window: 11}.Paginate(total, number, perPage)
}

s.Pagination.Paginator = wideWindow{}
```

Yours is used wherever the framework pages something — including the admin portal's list screens.
`view.Pages{Window: N}` is the built-in, so you can wrap it instead of starting over.

`PerPage: 0` still wins over a custom paginator: off means off.

## Where it applies

The admin's resource lists and the [generic views](07-views.md) both page through this, so one
setting covers every table in the project.

## Next

[Views →](07-views.md)
