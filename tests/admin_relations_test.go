package tests

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

type Shelf struct {
	ID   string `gorm:"primaryKey;size:64"`
	Name string `gorm:"size:100;not null"`
}

type Volume struct {
	ID      string  `gorm:"primaryKey;size:64"`
	Title   string  `gorm:"size:200;not null"`
	ShelfID *string `gorm:"size:64;index"`
	Shelf   Shelf
}

type volumeResource struct{}

func (volumeResource) Entity() any           { return Volume{} }
func (volumeResource) ListColumns() []string { return []string{"title", "shelf_id"} }

type shelfResource struct{}

func (shelfResource) Entity() any { return Shelf{} }

func volumePortal(t *testing.T) (*app.App, *client) {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }))
	a.RegisterModel(model.Of(Shelf{}), model.Of(Volume{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, shelf := range []Shelf{{ID: "s1", Name: "Fiction"}, {ID: "s2", Name: "Reference"}} {
		if err := handle.Create(&shelf).Error; err != nil {
			t.Fatal(err)
		}
	}
	shelf := "s1"
	if err := handle.Create(&Volume{ID: "v1", Title: "A Novel", ShelfID: &shelf}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(shelfResource{}, volumeResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return a, c
}

func TestForeignKeysRenderAsASelect(t *testing.T) {
	_, c := volumePortal(t)

	body := c.get("/admin/volumes/new").Body.String()
	if !strings.Contains(body, `<select name="shelf_id"`) {
		t.Fatalf("a foreign key should render as a select:\n%s", body)
	}
	if strings.Contains(body, `<input type="text" name="shelf_id"`) {
		t.Error("it must not also render as a text box")
	}
	for _, want := range []string{">Fiction<", ">Reference<"} {
		if !strings.Contains(body, want) {
			t.Errorf("the select should offer %q", want)
		}
	}
	if !strings.Contains(body, `<option value="">—</option>`) {
		t.Error("a nullable key should offer an empty choice")
	}
}

func TestTheCurrentRelationIsSelected(t *testing.T) {
	_, c := volumePortal(t)

	body := c.get("/admin/volumes/v1").Body.String()
	selected := regexp.MustCompile(`<option value="s1" selected>([^<]+)</option>`)
	if match := selected.FindStringSubmatch(body); match == nil {
		t.Errorf("the stored shelf should be preselected:\n%s", body)
	} else if strings.TrimSpace(match[1]) != "Fiction" {
		t.Errorf("selected label = %q", match[1])
	}
}

func TestTheListShowsTheLabelNotTheKey(t *testing.T) {
	_, c := volumePortal(t)

	body := c.get("/admin/volumes").Body.String()
	if !strings.Contains(body, "Fiction") {
		t.Errorf("the list should resolve the relation to its label:\n%s", body)
	}
	if strings.Contains(body, "<td>s1</td>") {
		t.Error("the raw key should not be shown when a label exists")
	}
}

func TestSavingThroughTheSelectStoresTheKey(t *testing.T) {
	a, c := volumePortal(t)

	rec := c.do(http.MethodPost, "/admin/volumes/new", url.Values{
		"csrf_token": {c.token("/admin/volumes/new")},
		"title":      {"Another"},
		"shelf_id":   {"s2"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body: %s", rec.Code, rec.Body.String())
	}

	records, _ := a.Store()
	schema, _ := a.Describe(Volume{})
	page, err := records.List(t.Context(), schema, model.Query{
		Filters: []model.Filter{{Column: "title", Op: model.Eq, Value: "Another"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("the new volume was not stored: %d rows", page.Total)
	}
	if got := page.Records[0].String("shelf_id"); got != "s2" {
		t.Errorf("shelf_id = %q, want s2", got)
	}
	if page.Records[0].String("id") == "" {
		t.Error("the framework should have generated a key")
	}
}

func TestAResourceWithoutRelationsIsUnaffected(t *testing.T) {
	_, c := volumePortal(t)

	body := c.get("/admin/shelves/new").Body.String()
	if strings.Contains(body, "<select name=") {
		t.Errorf("a model with no foreign key should render no select:\n%s", body)
	}
}
