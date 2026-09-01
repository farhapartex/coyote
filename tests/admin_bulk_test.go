package tests

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

var archived []string

type archivableBook struct{ bookResource }

func (archivableBook) Slug() string { return "archivable" }

func (archivableBook) Actions() []admin.Action {
	return []admin.Action{{
		Name:  "archive",
		Label: "Archive",
		Run: func(r *http.Request, ids []string) (int, error) {
			archived = append(archived, ids...)
			return len(ids), nil
		},
	}, {
		Name:  "explode",
		Label: "Explode",
		Run: func(r *http.Request, ids []string) (int, error) {
			return 0, errors.New("that did not work")
		},
	}}
}

func bulkPortal(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *client) {
	t.Helper()
	base := func(s *settings.Settings) {
		s.Admin.SiteName = "Test admin"
		s.Pagination.PerPage = 50
	}
	a := app.NewFrom(devSettings(t, append([]func(*settings.Settings){base}, fns...)...))
	a.RegisterModel(model.Of(Book{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range []Book{
		{ID: "b1", Title: "One", Published: true},
		{ID: "b2", Title: "Two"},
		{ID: "b3", Title: "Three"},
	} {
		if err := handle.Create(&book).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(bookResource{}, archivableBook{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return a, c
}

func countBooks(t *testing.T, a *app.App) int64 {
	t.Helper()
	records, _ := a.Store()
	schema, _ := a.Describe(Book{})
	total, err := records.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

func TestBulkDeleteRemovesTheSelectedRows(t *testing.T) {
	a, c := bulkPortal(t)

	body := c.get("/admin/books").Body.String()
	if !strings.Contains(body, `name="ids"`) {
		t.Fatalf("the list should offer selection checkboxes:\n%s", body)
	}
	if !strings.Contains(body, `<option value="delete">Delete</option>`) {
		t.Error("delete should be offered to someone who may delete")
	}

	rec := c.do(http.MethodPost, "/admin/books/bulk", url.Values{
		"csrf_token": {c.token("/admin/books")},
		"action":     {"delete"},
		"ids":        {"b1", "b3"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("bulk delete: %d, body: %s", rec.Code, rec.Body.String())
	}

	if got := countBooks(t, a); got != 1 {
		t.Errorf("%d books left, want 1", got)
	}
}

func TestBulkWithNothingSelectedSaysSo(t *testing.T) {
	a, c := bulkPortal(t)

	rec := c.do(http.MethodPost, "/admin/books/bulk", url.Values{
		"csrf_token": {c.token("/admin/books")},
		"action":     {"delete"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := countBooks(t, a); got != 3 {
		t.Errorf("nothing should have been deleted, %d left", got)
	}
	if body := c.get("/admin/books").Body.String(); !strings.Contains(body, "Nothing was selected") {
		t.Error("the flash should explain why nothing happened")
	}
}

func TestCustomActionsRun(t *testing.T) {
	archived = nil
	_, c := bulkPortal(t)

	body := c.get("/admin/archivable").Body.String()
	if !strings.Contains(body, `<option value="archive">Archive</option>`) {
		t.Errorf("a declared action should appear:\n%s", body)
	}

	rec := c.do(http.MethodPost, "/admin/archivable/bulk", url.Values{
		"csrf_token": {c.token("/admin/archivable")},
		"action":     {"archive"},
		"ids":        {"b1", "b2"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Join(archived, ",") != "b1,b2" {
		t.Errorf("the action received %v", archived)
	}
	if body := c.get("/admin/archivable").Body.String(); !strings.Contains(body, "Archive applied to 2") {
		t.Error("the result should be reported")
	}
}

func TestAFailingActionReportsAndChangesNothing(t *testing.T) {
	a, c := bulkPortal(t)

	rec := c.do(http.MethodPost, "/admin/archivable/bulk", url.Values{
		"csrf_token": {c.token("/admin/archivable")},
		"action":     {"explode"},
		"ids":        {"b1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := countBooks(t, a); got != 3 {
		t.Errorf("a failing action should not delete anything, %d left", got)
	}
	if body := c.get("/admin/archivable").Body.String(); !strings.Contains(body, "that did not work") {
		t.Error("the action's own error should reach the user, not a generic message")
	}
}

func TestAnUnknownActionIsRefused(t *testing.T) {
	a, c := bulkPortal(t)

	rec := c.do(http.MethodPost, "/admin/books/bulk", url.Values{
		"csrf_token": {c.token("/admin/books")},
		"action":     {"drop-everything"},
		"ids":        {"b1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := countBooks(t, a); got != 3 {
		t.Errorf("an unknown action must do nothing, %d left", got)
	}
}

func TestBulkDeleteNeedsThePermission(t *testing.T) {
	a, _ := bulkPortal(t)
	if _, err := a.SyncPermissions(); err != nil {
		t.Fatal(err)
	}
	helper, err := a.Auth.CreateUser(auth.NewUser{Username: "helper", Password: "unrelated-and-long", IsStaff: true})
	if err != nil {
		t.Fatal(err)
	}
	grant(t, a, helper.ID, "books.read")

	staff := newClient(t, a.Handler())
	staff.login("/admin/login", "helper", "unrelated-and-long")

	if body := staff.get("/admin/books").Body.String(); strings.Contains(body, `<option value="delete">`) {
		t.Error("someone without delete should not be offered it")
	}

	rec := staff.do(http.MethodPost, "/admin/books/bulk", url.Values{
		"csrf_token": {staff.token("/admin/books")},
		"action":     {"delete"},
		"ids":        {"b1"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if got := countBooks(t, a); got != 3 {
		t.Errorf("nothing should have been deleted, %d left", got)
	}
}

func staffWithGrants(t *testing.T, a *app.App, codenames ...string) *client {
	t.Helper()
	if _, err := a.SyncPermissions(); err != nil {
		t.Fatal(err)
	}
	helper, err := a.Auth.CreateUser(auth.NewUser{
		Username: "helper", Password: "unrelated-and-long", IsStaff: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(codenames) > 0 {
		grant(t, a, helper.ID, codenames...)
	}
	staff := newClient(t, a.Handler())
	staff.login("/admin/login", "helper", "unrelated-and-long")
	return staff
}

func TestACustomActionNeedsItsPermission(t *testing.T) {
	a, _ := bulkPortal(t)
	archived = nil
	staff := staffWithGrants(t, a, "books.read")

	rec := staff.do(http.MethodPost, "/admin/archivable/bulk", url.Values{
		"csrf_token": {staff.token("/admin/archivable")},
		"action":     {"archive"},
		"ids":        {"b1"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; a custom action defaults to needing update", rec.Code)
	}
	if len(archived) != 0 {
		t.Errorf("the action ran anyway: %v", archived)
	}
}

func TestACustomActionRunsForSomeoneHoldingItsPermission(t *testing.T) {
	a, _ := bulkPortal(t)
	archived = nil
	staff := staffWithGrants(t, a, "books.read", "books.update")

	rec := staff.do(http.MethodPost, "/admin/archivable/bulk", url.Values{
		"csrf_token": {staff.token("/admin/archivable")},
		"action":     {"archive"},
		"ids":        {"b1", "b2"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if len(archived) != 2 {
		t.Errorf("archived = %v, want both rows", archived)
	}
}

func TestBulkNeedsReadToReachTheRouteAtAll(t *testing.T) {
	a, _ := bulkPortal(t)
	archived = nil
	staff := staffWithGrants(t, a)

	rec := staff.do(http.MethodPost, "/admin/archivable/bulk", url.Values{
		"csrf_token": {staff.token("/admin/")},
		"action":     {"archive"},
		"ids":        {"b1"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; no grant on the resource means the route is closed", rec.Code)
	}
	if len(archived) != 0 {
		t.Errorf("the action ran anyway: %v", archived)
	}
}

func TestBulkRefusesMoreRowsThanItWillTake(t *testing.T) {
	a, c := bulkPortal(t)

	ids := make([]string, 1001)
	for i := range ids {
		ids[i] = "b1"
	}
	rec := c.do(http.MethodPost, "/admin/books/bulk", url.Values{
		"csrf_token": {c.token("/admin/books")},
		"action":     {"delete"},
		"ids":        ids,
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := countBooks(t, a); got != 3 {
		t.Errorf("%d books left, want all 3 kept", got)
	}
}
