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

type Article struct {
	ID      string `gorm:"primaryKey;size:64"`
	Title   string `gorm:"size:200;not null"`
	Summary string `gorm:"size:2000"`
}

type articleResource struct{}

func (articleResource) Entity() any { return Article{} }

var textareaPattern = regexp.MustCompile(`(?s)<textarea[^>]*name="summary"[^>]*>(.*?)</textarea>`)

func formPortal(t *testing.T) *client {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) { s.Admin.SiteName = "Test admin" }))
	a.RegisterModel(model.Of(Article{}))
	syncSchema(t, a)
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(articleResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return c
}

func TestLongTextColumnsRenderAsATextarea(t *testing.T) {
	c := formPortal(t)
	body := c.get("/admin/articles/new").Body.String()

	match := textareaPattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("a column over 500 characters should render as a textarea:\n%s", body)
	}
	if !strings.Contains(body, `rows="3"`) {
		t.Error("textareas should default to three rows")
	}
	if strings.Contains(body, `<input type="text" name="summary"`) {
		t.Error("the column should not also render as a single-line input")
	}
	if !strings.Contains(body, `<input type="text" name="title"`) {
		t.Error("a short string column should stay a single-line input")
	}
}

func TestTextareaValuesSurviveARoundTrip(t *testing.T) {
	c := formPortal(t)
	summary := "First line\nSecond line with <angle> & ampersand"

	rec := c.do(http.MethodPost, "/admin/articles/new", url.Values{
		"csrf_token": {c.token("/admin/articles/new")},
		"title":      {"Release notes"},
		"summary":    {summary},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body: %s", rec.Code, rec.Body.String())
	}

	list := c.get("/admin/articles").Body.String()
	id := regexp.MustCompile(`/admin/articles/([0-9a-f-]{36})`).FindStringSubmatch(list)
	if id == nil {
		t.Fatal("the new article is not listed")
	}

	body := c.get("/admin/articles/" + id[1]).Body.String()
	match := textareaPattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatal("no textarea on the edit form")
	}
	if !strings.Contains(match[1], "First line") || !strings.Contains(match[1], "Second line") {
		t.Errorf("the stored text should sit inside the element, got %q", match[1])
	}
	if strings.Contains(body, `value="First line`) {
		t.Error("a textarea must carry its content as text, not a value attribute")
	}
	if !strings.Contains(match[1], "&lt;angle&gt;") {
		t.Errorf("the content should be escaped, got %q", match[1])
	}
}

func TestAdminFormsUseTheStyledFormClass(t *testing.T) {
	c := formPortal(t)

	for _, target := range []string{"/admin/articles/new", "/admin/users/new", "/admin/roles/new"} {
		body := c.get(target).Body.String()
		forms := strings.Count(body, "<form method=\"post\"")
		styled := strings.Count(body, `class="stack"`)
		if styled < 1 {
			t.Errorf("%s has %d forms but none carry class=\"stack\", so the inputs render unstyled", target, forms)
		}
	}
}
