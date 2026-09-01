package tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/view"
)

func TestDefaultPerPageIsTen(t *testing.T) {
	if got := settings.Default().Pagination.PerPage; got != view.DefaultPerPage {
		t.Errorf("default PerPage = %d, want the package default", got)
	}
	if view.DefaultPerPage != 10 {
		t.Errorf("DefaultPerPage = %d, want 10", view.DefaultPerPage)
	}
	if settings.Default().Pagination.Paginator != nil {
		t.Error("the default paginator should be nil so the built-in one is used")
	}
}

func TestPaginateSplitsATotal(t *testing.T) {
	page := view.Paginate(nil, 95, 1, 10)

	if !page.Enabled || page.Pages != 10 || page.Number != 1 {
		t.Errorf("page = %+v", page)
	}
	if page.Offset != 0 || page.Limit != 10 {
		t.Errorf("offset/limit = %d/%d", page.Offset, page.Limit)
	}
	if page.HasPrev || !page.HasNext || page.Next != 2 {
		t.Errorf("navigation = %+v", page)
	}

	middle := view.Paginate(nil, 95, 5, 10)
	if middle.Offset != 40 || !middle.HasPrev || !middle.HasNext {
		t.Errorf("middle = %+v", middle)
	}

	last := view.Paginate(nil, 95, 10, 10)
	if last.HasNext || last.Offset != 90 {
		t.Errorf("last = %+v", last)
	}
}

func TestPaginateClampsOutOfRangePages(t *testing.T) {
	for _, testcase := range []struct{ asked, want int }{
		{0, 1}, {-5, 1}, {1, 1}, {3, 3}, {99, 3},
	} {
		if got := view.Paginate(nil, 25, testcase.asked, 10).Number; got != testcase.want {
			t.Errorf("page %d resolved to %d, want %d", testcase.asked, got, testcase.want)
		}
	}
}

func TestZeroPerPageTurnsPaginationOff(t *testing.T) {
	page := view.Paginate(nil, 500, 3, 0)

	if page.Enabled {
		t.Error("PerPage 0 must disable pagination")
	}
	if page.Limit != 0 {
		t.Errorf("Limit = %d; zero means the store applies no limit", page.Limit)
	}
	if page.Offset != 0 || page.Number != 1 || page.Pages != 1 {
		t.Errorf("a disabled page should describe one page of everything: %+v", page)
	}
}

func TestEmptyResultStillHasOnePage(t *testing.T) {
	page := view.Paginate(nil, 0, 1, 10)

	if page.Pages != 1 || page.HasNext || page.HasPrev {
		t.Errorf("page = %+v", page)
	}
	if !page.Empty() || !page.Single() {
		t.Error("an empty result should report Empty and Single")
	}
}

func TestWindowFollowsTheCurrentPage(t *testing.T) {
	if got := view.Paginate(nil, 200, 1, 10).Numbers; fmt.Sprint(got) != "[1 2 3 4 5]" {
		t.Errorf("start window = %v", got)
	}
	if got := view.Paginate(nil, 200, 10, 10).Numbers; fmt.Sprint(got) != "[8 9 10 11 12]" {
		t.Errorf("middle window = %v", got)
	}
	if got := view.Paginate(nil, 200, 20, 10).Numbers; fmt.Sprint(got) != "[16 17 18 19 20]" {
		t.Errorf("end window = %v", got)
	}
	if got := view.Paginate(nil, 25, 1, 10).Numbers; fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("a short run should not pad: %v", got)
	}
}

type fixedPaginator struct{}

func (fixedPaginator) Paginate(total int64, number, perPage int) view.Page {
	return view.Page{Enabled: true, Number: 1, PerPage: 3, Total: total, Pages: 1, Limit: 3, Numbers: []int{1}}
}

func TestACustomPaginatorReplacesTheBuiltIn(t *testing.T) {
	page := view.Paginate(fixedPaginator{}, 500, 7, 10)

	if page.Limit != 3 || page.Pages != 1 {
		t.Errorf("the supplied paginator should decide: %+v", page)
	}

	if got := view.Paginate(fixedPaginator{}, 500, 7, 0); got.Enabled {
		t.Error("PerPage 0 must win over a custom paginator")
	}
}

type Row struct {
	ID   string `gorm:"primaryKey;size:64"`
	Name string `gorm:"size:100;not null"`
}

type rowResource struct{}

func (rowResource) Entity() any { return Row{} }

func listPortal(t *testing.T, rows int, fns ...func(*settings.Settings)) *client {
	t.Helper()
	base := func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }
	a := app.NewFrom(devSettings(t, append([]func(*settings.Settings){base}, fns...)...))
	a.RegisterModel(model.Of(Row{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if err := handle.Create(&Row{ID: fmt.Sprintf("row-%03d", i), Name: fmt.Sprintf("Row %d", i)}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(rowResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return c
}

func TestAdminListFollowsThePerPageSetting(t *testing.T) {
	c := listPortal(t, 25)

	body := c.get("/admin/rows").Body.String()
	if got := strings.Count(body, "/admin/rows/row-"); got != 10 {
		t.Errorf("first page showed %d rows, want the default of 10", got)
	}
	if !strings.Contains(body, "Page 1 of 3") {
		t.Error("the pager should report the page count")
	}

	last := c.get("/admin/rows?page=3").Body.String()
	if got := strings.Count(last, "/admin/rows/row-"); got != 5 {
		t.Errorf("last page showed %d rows, want 5", got)
	}
	if strings.Contains(last, ">Next</a>") {
		t.Error("the last page should not offer Next")
	}
}

func TestAdminListHonoursACustomPerPage(t *testing.T) {
	c := listPortal(t, 25, func(s *settings.Settings) { s.Pagination.PerPage = 5 })

	body := c.get("/admin/rows").Body.String()
	if got := strings.Count(body, "/admin/rows/row-"); got != 5 {
		t.Errorf("showed %d rows, want the configured 5", got)
	}
	if !strings.Contains(body, "Page 1 of 5") {
		t.Error("the page count should follow the setting")
	}
}

func TestAdminListWithPaginationOffShowsEverything(t *testing.T) {
	c := listPortal(t, 25, func(s *settings.Settings) { s.Pagination.PerPage = 0 })

	body := c.get("/admin/rows").Body.String()
	if got := strings.Count(body, "/admin/rows/row-"); got != 25 {
		t.Errorf("showed %d rows, want all 25", got)
	}
	if strings.Contains(body, `class="pager"`) {
		t.Error("no controls should render when pagination is off")
	}
}

func TestNegativePerPageIsRejected(t *testing.T) {
	_, err := settings.New(prodSettings(func(s *settings.Settings) { s.Pagination.PerPage = -1 })...)
	if err == nil {
		t.Fatal("a negative PerPage should be rejected")
	}
	mustContain(t, problemsOf(t, err), "Pagination.PerPage")
}
