package tests

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/upload"
)

type Poster struct {
	ID    string     `gorm:"primaryKey;size:64"`
	Title string     `gorm:"size:200;not null"`
	Image upload.Ref `gorm:"size:200" coyote:"path=posters/images,accept=image/*"`
	Plain string     `gorm:"size:200" coyote:"file,path=posters/files"`
}

type posterResource struct{}

func (posterResource) Entity() any { return Poster{} }

func filePortal(t *testing.T) (*app.App, *client) {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Admin.SiteName = "Test admin"
		s.Uploads.Enabled = true
		s.Uploads.Path = "fallback"
		s.Uploads.Allowed = []string{"image/png", "text/plain"}
		s.Uploads.Serve = true
	}))
	a.RegisterModel(model.Of(Poster{}))
	syncSchema(t, a)
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "unrelated-and-long"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	portal.MustManage(posterResource{})

	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "unrelated-and-long")
	return a, c
}

func TestFileColumnsAreDetectedFromTheModel(t *testing.T) {
	a, _ := filePortal(t)

	schema, err := a.Describe(Poster{})
	if err != nil {
		t.Fatal(err)
	}

	image, _ := schema.Field("image")
	if image.Kind != model.KindFile {
		t.Errorf("an upload.Ref column should be KindFile, got %q", image.Kind)
	}
	if image.UploadPath != "posters/images" {
		t.Errorf("UploadPath = %q", image.UploadPath)
	}
	if image.Accept != "image/*" {
		t.Errorf("Accept = %q", image.Accept)
	}

	plain, _ := schema.Field("plain")
	if plain.Kind != model.KindFile {
		t.Errorf("a tagged string column should be KindFile, got %q", plain.Kind)
	}

	title, _ := schema.Field("title")
	if title.Kind == model.KindFile {
		t.Error("an ordinary column must not become a file field")
	}
}

func TestAdminRendersAFileInput(t *testing.T) {
	_, c := filePortal(t)

	body := c.get("/admin/posters/new").Body.String()
	if !strings.Contains(body, `enctype="multipart/form-data"`) {
		t.Error("a form with a file field needs the multipart encoding")
	}
	if !strings.Contains(body, `<input type="file" name="image"`) {
		t.Errorf("no file input rendered:\n%s", body)
	}
	if !strings.Contains(body, `accept="image/*"`) {
		t.Error("the accept tag should reach the input")
	}
	if strings.Contains(body, `<input type="text" name="image"`) {
		t.Error("a file column must not also render as a text box")
	}
}

func uploadForm(t *testing.T, token, title, field string, body []byte, filename string) (string, *bytes.Buffer) {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	writer.WriteField("csrf_token", token)
	writer.WriteField("title", title)
	if field != "" {
		part, err := writer.CreateFormFile(field, filename)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(body)
	}
	writer.Close()
	return writer.FormDataContentType(), buffer
}

func (c *client) postMultipart(target, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Content-Type", contentType)
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	if cookies := rec.Result().Cookies(); len(cookies) > 0 {
		c.cookie = cookies[0]
	}
	return rec
}

func TestUploadingThroughTheAdminStoresUnderTheFieldPath(t *testing.T) {
	a, c := filePortal(t)

	kind, body := uploadForm(t, c.token("/admin/posters/new"), "Launch", "image", pngBytes(t, 4, 4), "poster.png")
	if rec := c.postMultipart("/admin/posters/new", kind, body); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body: %s", rec.Code, rec.Body.String())
	}

	store, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	schema, _ := a.Describe(Poster{})
	page, err := store.List(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("stored %d posters", page.Total)
	}

	ref := upload.Ref(page.Records[0].String("image"))
	if !strings.HasPrefix(string(ref), "posters/images/") {
		t.Errorf("ref = %q, want the field path at the media root", ref)
	}
	if !ref.Committed() {
		t.Errorf("the file should be committed, got %q", ref)
	}
	if !a.Uploads.Storage().Exists(t.Context(), string(ref)) {
		t.Error("the bytes should be on disk")
	}
}

func TestFieldPathFallsBackToSettingsThenDefault(t *testing.T) {
	a, _ := filePortal(t)

	if got := a.Uploads.PathFor("posters/images"); got != "posters/images" {
		t.Errorf("a field path should win, got %q", got)
	}
	if got := a.Uploads.PathFor(""); got != "fallback" {
		t.Errorf("an empty field path should fall back to settings, got %q", got)
	}

	bare := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Uploads.Enabled = true
	}))
	if got := bare.Uploads.PathFor(""); got != "" {
		t.Errorf("with nothing set the framework default applies, got %q", got)
	}
	if got := bare.Uploads.PathFor("../../escape"); got != "escape" {
		t.Errorf("a path must not climb out of the media root, got %q", got)
	}
}

func TestEditKeepsTheFileWhenNoNewOneIsSent(t *testing.T) {
	a, c := filePortal(t)

	kind, body := uploadForm(t, c.token("/admin/posters/new"), "Launch", "image", pngBytes(t, 4, 4), "poster.png")
	c.postMultipart("/admin/posters/new", kind, body)

	store, _ := a.Store()
	schema, _ := a.Describe(Poster{})
	page, _ := store.List(t.Context(), schema, model.Query{})
	id := page.Records[0].String("id")
	original := page.Records[0].String("image")

	kind, body = uploadForm(t, c.token("/admin/posters/"+id), "Renamed", "", nil, "")
	if rec := c.postMultipart("/admin/posters/"+id, kind, body); rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d, body: %s", rec.Code, rec.Body.String())
	}

	record, err := store.Find(t.Context(), schema, id)
	if err != nil {
		t.Fatal(err)
	}
	if record.String("title") != "Renamed" {
		t.Errorf("title = %q", record.String("title"))
	}
	if record.String("image") != original {
		t.Errorf("the file should survive an edit that did not touch it: %q vs %q", record.String("image"), original)
	}
}

func TestARejectedUploadIsReportedOnTheForm(t *testing.T) {
	_, c := filePortal(t)

	kind, body := uploadForm(t, c.token("/admin/posters/new"), "Launch", "image", []byte("%PDF-1.4 nope"), "doc.pdf")
	rec := c.postMultipart("/admin/posters/new", kind, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not an accepted file type") {
		t.Errorf("the form should explain the rejection:\n%s", rec.Body.String())
	}
}

func TestUploadedFileIsServedBack(t *testing.T) {
	a, c := filePortal(t)

	kind, body := uploadForm(t, c.token("/admin/posters/new"), "Launch", "image", pngBytes(t, 4, 4), "poster.png")
	c.postMultipart("/admin/posters/new", kind, body)

	store, _ := a.Store()
	schema, _ := a.Describe(Poster{})
	page, _ := store.List(t.Context(), schema, model.Query{})
	ref := upload.Ref(page.Records[0].String("image"))

	rec := c.get(a.MediaURL(ref))
	if rec.Code != http.StatusOK {
		t.Fatalf("serving: %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}

	form := c.get("/admin/posters/" + page.Records[0].String("id")).Body.String()
	if !strings.Contains(form, "Current file") {
		t.Error("the edit form should link to the stored file")
	}
}
