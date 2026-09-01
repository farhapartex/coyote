package tests

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/view"
)

type Note struct {
	ID    string `gorm:"primaryKey;size:64"`
	Title string `gorm:"size:200;not null"`
	Body  string `gorm:"size:400"`
}

func genericApp(t *testing.T, seed int) (*app.App, model.Store, *model.Schema) {
	t.Helper()

	pages := fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{Data: []byte(
			`{{define "base.html"}}<html><body>{{block "content" .}}{{end}}</body></html>{{end}}`)},
		"pages/list.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<h1>{{.Total}}</h1>{{range .Records}}<p>{{.String "title"}}</p>{{end}}` +
				`{{with .Page}}<span>page {{.Number}} of {{.Pages}}</span>{{end}}{{end}}`)},
		"pages/detail.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<h1>{{.Record.String "title"}}</h1>{{end}}`)},
		"pages/form.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}{{if .Problems}}<ul>{{range $f, $p := .Problems}}<li>{{$f}}</li>{{end}}</ul>{{end}}` +
				`<form method="post"></form>{{end}}`)},
	}

	a := app.NewFrom(devSettings(t, withoutCSRF, func(s *settings.Settings) {
		s.Templates.FS = pages
		s.Templates.Layout = "layouts/base.html"
		s.Templates.Shared = []string{"layouts/*.html"}
	}))
	a.RegisterModel(model.Of(Note{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for i := range seed {
		if err := handle.Create(&Note{ID: fmt.Sprintf("note-%02d", i), Title: fmt.Sprintf("Note %d", i)}).Error; err != nil {
			t.Fatal(err)
		}
	}

	store, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := a.Describe(Note{})
	if err != nil {
		t.Fatal(err)
	}
	return a, store, schema
}

func TestGenericListPagesAndRenders(t *testing.T) {
	a, store, schema := genericApp(t, 12)

	a.Get("/notes", view.List(view.Options{
		Store: store, Schema: schema, Template: "pages/list.html",
		Renderer: a, PerPage: 5, Order: "title asc",
	}))

	body := newClient(t, a.Handler()).get("/notes").Body.String()
	if strings.Count(body, "<p>") != 5 {
		t.Errorf("first page rendered %d records, want 5", strings.Count(body, "<p>"))
	}
	if !strings.Contains(body, "page 1 of 3") {
		t.Errorf("pagination missing from %s", body)
	}

	third := newClient(t, a.Handler()).get("/notes?page=3").Body.String()
	if strings.Count(third, "<p>") != 2 {
		t.Errorf("last page rendered %d records, want 2", strings.Count(third, "<p>"))
	}
}

func TestGenericListWithoutPaginationShowsEverything(t *testing.T) {
	a, store, schema := genericApp(t, 12)

	a.Get("/notes", view.List(view.Options{
		Store: store, Schema: schema, Template: "pages/list.html", Renderer: a,
	}))

	body := newClient(t, a.Handler()).get("/notes").Body.String()
	if strings.Count(body, "<p>") != 12 {
		t.Errorf("rendered %d records, want all 12", strings.Count(body, "<p>"))
	}
}

func TestGenericDetail(t *testing.T) {
	a, store, schema := genericApp(t, 3)

	a.Get("/notes/{id}", view.Detail(view.Options{
		Store: store, Schema: schema, Template: "pages/detail.html", Renderer: a,
	}))

	c := newClient(t, a.Handler())
	if body := c.get("/notes/note-01").Body.String(); !strings.Contains(body, "Note 1") {
		t.Errorf("body = %s", body)
	}
	if rec := c.get("/notes/absent"); rec.Code != http.StatusNotFound {
		t.Errorf("a missing record should 404, got %d", rec.Code)
	}
}

func TestGenericCreateValidatesAndInserts(t *testing.T) {
	a, store, schema := genericApp(t, 0)

	a.Any("/notes/new", view.Create(view.Options{
		Store: store, Schema: schema, Template: "pages/form.html",
		Renderer: a, Redirect: "/notes",
	}))

	c := newClient(t, a.Handler())
	if rec := c.get("/notes/new"); rec.Code != http.StatusOK {
		t.Fatalf("GET form: %d", rec.Code)
	}

	rec := c.do(http.MethodPost, "/notes/new", url.Values{"body": {"no title"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a missing required field should be 422, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<li>title</li>") {
		t.Errorf("the failing field should be reported: %s", rec.Body.String())
	}

	rec = c.do(http.MethodPost, "/notes/new", url.Values{"title": {"Fresh"}, "body": {"words"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("a valid post should redirect, got %d: %s", rec.Code, rec.Body.String())
	}

	page, err := store.List(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Errorf("stored %d records, want 1", page.Total)
	}
}

func TestGenericUpdateAndDelete(t *testing.T) {
	a, store, schema := genericApp(t, 2)

	a.Any("/notes/{id}/edit", view.Update(view.Options{
		Store: store, Schema: schema, Template: "pages/form.html",
		Renderer: a, Redirect: "/notes",
	}))
	a.Post("/notes/{id}/delete", view.Delete(view.Options{
		Store: store, Schema: schema, Renderer: a, Redirect: "/notes",
	}))

	c := newClient(t, a.Handler())
	if rec := c.do(http.MethodPost, "/notes/note-00/edit", url.Values{"title": {"Renamed"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rec.Code)
	}
	record, err := store.Find(t.Context(), schema, "note-00")
	if err != nil {
		t.Fatal(err)
	}
	if record.String("title") != "Renamed" {
		t.Errorf("title = %q", record.String("title"))
	}

	if rec := c.do(http.MethodPost, "/notes/note-01/delete", nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if _, err := store.Find(t.Context(), schema, "note-01"); err == nil {
		t.Error("the record should be gone")
	}
}

func TestGenericViewsRespectAllowAndFilter(t *testing.T) {
	a, store, schema := genericApp(t, 6)

	a.Get("/private", view.List(view.Options{
		Store: store, Schema: schema, Template: "pages/list.html", Renderer: a,
		Allow: func(*http.Request) bool { return false },
	}))
	a.Get("/filtered", view.List(view.Options{
		Store: store, Schema: schema, Template: "pages/list.html", Renderer: a,
		Filter: func(_ *http.Request, q model.Query) model.Query {
			q.Limit = 2
			return q
		},
	}))

	c := newClient(t, a.Handler())
	if rec := c.get("/private"); rec.Code != http.StatusForbidden {
		t.Errorf("Allow should gate the view, got %d", rec.Code)
	}
	if body := c.get("/filtered").Body.String(); strings.Count(body, "<p>") != 2 {
		t.Errorf("Filter should shape the query, rendered %d", strings.Count(body, "<p>"))
	}
}

func TestGenericViewsAddCustomData(t *testing.T) {
	a, store, schema := genericApp(t, 1)

	a.Get("/notes", view.List(view.Options{
		Store: store, Schema: schema, Template: "pages/list.html", Renderer: a,
		Data: func(_ *http.Request, data view.Data) view.Data {
			return data.Set("Total", int64(99))
		},
	}))

	if body := newClient(t, a.Handler()).get("/notes").Body.String(); !strings.Contains(body, "<h1>99</h1>") {
		t.Errorf("custom data should reach the template: %s", body)
	}
}

func TestGenericCreateOnlyBindsTheNamedFields(t *testing.T) {
	a, store, schema := genericApp(t, 0)

	a.Any("/notes/new", view.Create(view.Options{
		Store: store, Schema: schema, Template: "pages/form.html",
		Renderer: a, Redirect: "/notes",
		Fields: []string{"title"},
	}))

	rec := newClient(t, a.Handler()).do(http.MethodPost, "/notes/new", url.Values{
		"title": {"Public note"}, "body": {"set by hand"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}

	page, err := store.List(t.Context(), schema, model.Query{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	row := page.Records[0]
	if row.String("title") != "Public note" {
		t.Errorf("title = %q, the named field should bind", row.String("title"))
	}
	if row.String("body") != "" {
		t.Errorf("body = %q, a column outside Fields must not bind from the request", row.String("body"))
	}
}

func TestGenericCreateHonoursExclude(t *testing.T) {
	a, store, schema := genericApp(t, 0)

	a.Any("/notes/new", view.Create(view.Options{
		Store: store, Schema: schema, Template: "pages/form.html",
		Renderer: a, Redirect: "/notes",
		Exclude: []string{"body"},
	}))

	rec := newClient(t, a.Handler()).do(http.MethodPost, "/notes/new", url.Values{
		"title": {"Public note"}, "body": {"set by hand"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}

	page, err := store.List(t.Context(), schema, model.Query{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if body := page.Records[0].String("body"); body != "" {
		t.Errorf("body = %q, an excluded column must not bind", body)
	}
}
