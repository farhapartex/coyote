package tests

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFAcceptsATokenFromAMultipartForm(t *testing.T) {
	a := newTestApp(t)
	a.Post("/upload", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }, a.CSRF)
	a.Get("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<input name="csrf_token" value="` + a.Sessions.CSRFToken(r) + `">`))
	})

	c := newClient(t, a.Handler())
	token := c.token("/form")

	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	writer.WriteField("csrf_token", token)
	part, err := writer.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("hello"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", buffer)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(c.cookie)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("a multipart form carrying a valid token should pass, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFStillRefusesAMultipartFormWithoutAToken(t *testing.T) {
	a := newTestApp(t)
	a.Post("/upload", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }, a.CSRF)

	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	writer.WriteField("csrf_token", "not-the-token")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", buffer)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("a wrong token must still be refused, got %d", rec.Code)
	}
}
