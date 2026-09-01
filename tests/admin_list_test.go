package tests

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

type Book struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:200;not null"`
	Author    string `gorm:"size:100"`
	Published bool   `gorm:"index"`
	Pages     int
}

type bookResource struct{}

func (bookResource) Entity() any             { return Book{} }
func (bookResource) SearchColumns() []string { return []string{"title", "author"} }
func (bookResource) FilterColumns() []string { return []string{"published"} }
func (bookResource) ListColumns() []string   { return []string{"title", "author", "pages"} }

type plainBook struct{}

func (plainBook) Entity() any           { return Book{} }
func (plainBook) Slug() string          { return "plain" }
func (plainBook) ListColumns() []string { return []string{"title"} }

func bookPortal(t *testing.T) (*app.App, *client) {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Admin.SiteName = "Test admin"
		s.Pagination.PerPage = 50
	}))
	a.RegisterModel(model.Of(Book{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range []Book{
		{ID: "b1", Title: "Go in Practice", Author: "Alice", Published: true, Pages: 300},
		{ID: "b2", Title: "Rust Basics", Author: "Bob", Published: false, Pages: 120},
		{ID: "b3", Title: "Advanced Go", Author: "Carol", Published: true, Pages: 450},
	} {
		if err := handle.Create(&book).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(bookResource{}, plainBook{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return a, c
}

var bodyPattern = regexp.MustCompile(`(?s)<tbody>(.*?)</tbody>`)
var rowPattern = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
var cellPattern = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)

func rowTitles(body string) []string {
	section := bodyPattern.FindStringSubmatch(body)
	if section == nil {
		return nil
	}

	out := []string{}
	for _, row := range rowPattern.FindAllStringSubmatch(section[1], -1) {
		for _, cell := range cellPattern.FindAllStringSubmatch(row[1], -1) {
			value := strings.TrimSpace(cell[1])
			if value == "" || value == "—" || strings.Contains(value, "<") {
				continue
			}
			out = append(out, value)
			break
		}
	}
	return out
}

func TestSearchNarrowsTheList(t *testing.T) {
	_, c := bookPortal(t)

	body := c.get("/admin/books").Body.String()
	if !strings.Contains(body, `name="q"`) {
		t.Error("a searchable resource should render a search box")
	}

	found := c.get("/admin/books?q=Go").Body.String()
	titles := strings.Join(rowTitles(found), " ")
	if !strings.Contains(titles, "Go in Practice") || !strings.Contains(titles, "Advanced Go") {
		t.Errorf("search should match both Go titles, got %q", titles)
	}
	if strings.Contains(titles, "Rust") {
		t.Errorf("search should exclude non-matches, got %q", titles)
	}

	if !strings.Contains(found, "(2)") {
		t.Error("the count should reflect the search, not the whole table")
	}
}

func TestSearchAlsoLooksAtOtherDeclaredColumns(t *testing.T) {
	_, c := bookPortal(t)

	titles := strings.Join(rowTitles(c.get("/admin/books?q=Bob").Body.String()), " ")
	if !strings.Contains(titles, "Rust Basics") {
		t.Errorf("searching an author should work, got %q", titles)
	}
}

func TestSearchIsAbsentUnlessDeclared(t *testing.T) {
	_, c := bookPortal(t)

	body := c.get("/admin/plain").Body.String()
	if strings.Contains(body, `name="q"`) {
		t.Error("a resource that declares no search columns should not offer a search box")
	}

	all := rowTitles(c.get("/admin/plain?q=Go").Body.String())
	if len(all) != 3 {
		t.Errorf("an ignored search must not filter anything, got %v", all)
	}
}

func TestSearchTreatsWildcardsLiterally(t *testing.T) {
	_, c := bookPortal(t)

	titles := rowTitles(c.get("/admin/books?q=%25").Body.String())
	if len(titles) != 0 {
		t.Errorf("a percent sign should be searched literally, got %v", titles)
	}
}

func TestFiltersNarrowByColumn(t *testing.T) {
	_, c := bookPortal(t)

	body := c.get("/admin/books").Body.String()
	if !strings.Contains(body, `name="filter.published"`) {
		t.Error("a filterable column should render a dropdown")
	}

	published := strings.Join(rowTitles(c.get("/admin/books?filter.published=yes").Body.String()), " ")
	if strings.Contains(published, "Rust") {
		t.Errorf("filtering to published should exclude drafts, got %q", published)
	}

	drafts := rowTitles(c.get("/admin/books?filter.published=no").Body.String())
	if len(drafts) == 0 || !strings.Contains(strings.Join(drafts, " "), "Rust") {
		t.Errorf("filtering to unpublished should keep the draft, got %v", drafts)
	}
}

func TestSortableHeadersChangeTheOrder(t *testing.T) {
	_, c := bookPortal(t)

	body := c.get("/admin/books").Body.String()
	if !strings.Contains(body, "?sort=title") {
		t.Errorf("headers should link to a sort:\n%s", body)
	}

	ascending := rowTitles(c.get("/admin/books?sort=pages").Body.String())
	descending := rowTitles(c.get("/admin/books?sort=-pages").Body.String())
	if len(ascending) != 3 || len(descending) != 3 {
		t.Fatalf("expected three rows, got %v and %v", ascending, descending)
	}
	if ascending[0] == descending[0] {
		t.Errorf("ascending and descending should differ: %v vs %v", ascending, descending)
	}
	if ascending[0] != "Rust Basics" {
		t.Errorf("ascending by pages should start with the shortest book, got %v", ascending)
	}

	sorted := c.get("/admin/books?sort=title").Body.String()
	if !strings.Contains(sorted, "?sort=-title") {
		t.Error("clicking a sorted header should offer the reverse")
	}
}

func TestAnInjectedSortIsIgnoredByTheAdmin(t *testing.T) {
	_, c := bookPortal(t)

	rec := c.get("/admin/books?sort=title;drop+table+books")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(rowTitles(rec.Body.String())) != 3 {
		t.Error("a hostile sort should be ignored, not break the page")
	}
}

func TestSearchAndSortCombine(t *testing.T) {
	_, c := bookPortal(t)

	titles := rowTitles(c.get("/admin/books?q=Go&sort=-pages").Body.String())
	if len(titles) != 2 {
		t.Fatalf("expected two matches, got %v", titles)
	}
	if titles[0] != "Advanced Go" {
		t.Errorf("the search should be sorted too, got %v", titles)
	}

	body := c.get("/admin/books?q=Go&sort=title").Body.String()
	if !strings.Contains(body, "q=Go") {
		t.Error("sort links should keep the search term")
	}
}

func TestEmptySearchSaysSo(t *testing.T) {
	_, c := bookPortal(t)

	body := c.get("/admin/books?q=nothingmatches").Body.String()
	if !strings.Contains(body, "Nothing matches that search") {
		t.Errorf("an empty search should explain itself:\n%s", body)
	}
}
