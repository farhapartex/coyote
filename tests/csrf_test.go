package tests

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/settings"
)

func TestCSRFGuardsEveryRouteWithoutBeingAskedTo(t *testing.T) {
	a := newTestApp(t)
	a.Post("/orders", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("placed")) })

	rec := newClient(t, a.Handler()).do(http.MethodPost, "/orders", url.Values{"total": {"9"}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; a route the developer wrote must be guarded by default", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "placed") {
		t.Error("the handler ran on a request carrying no token")
	}
}

func TestCSRFAcceptsATokenOnARouteTheDeveloperNeverGuarded(t *testing.T) {
	a := newTestApp(t)
	a.Post("/orders", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("placed")) })
	a.Get("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<input name="csrf_token" value="` + a.Sessions.CSRFToken(r) + `">`))
	})

	c := newClient(t, a.Handler())
	rec := c.do(http.MethodPost, "/orders", url.Values{"csrf_token": {c.token("/form")}})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFLetsTheExemptPrefixesThrough(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Security.CSRFExempt = []string{"/hooks/"}
	})
	a.Post("/hooks/stripe", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("received")) })
	a.Post("/orders", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("placed")) })

	c := newClient(t, a.Handler())
	if rec := c.do(http.MethodPost, "/hooks/stripe", nil); rec.Code != http.StatusOK {
		t.Errorf("an exempt path answered %d, want 200", rec.Code)
	}
	if rec := c.do(http.MethodPost, "/orders", nil); rec.Code != http.StatusForbidden {
		t.Errorf("a path outside the exempt list answered %d, want 403", rec.Code)
	}
}

func TestCSRFCanBeTurnedOffWholesale(t *testing.T) {
	a := newTestApp(t, withoutCSRF)
	a.Post("/orders", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("placed")) })

	if rec := newClient(t, a.Handler()).do(http.MethodPost, "/orders", nil); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 once Security.CSRF is off", rec.Code)
	}
}

func TestCSRFValidationRejectsTurningItOffInProduction(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Environment = settings.Production
		s.Security.CSRF = false
	})...)
	if err == nil || !strings.Contains(err.Error(), "Security.CSRF is off") {
		t.Errorf("error = %v, want a complaint about CSRF being off in production", err)
	}
}

func TestCSRFValidationRejectsARelativeExemptPath(t *testing.T) {
	_, err := settings.New(append(prodSettings(), func(s *settings.Settings) {
		s.Security.CSRFExempt = []string{"hooks/"}
	})...)
	if err == nil || !strings.Contains(err.Error(), "CSRFExempt paths must start with") {
		t.Errorf("error = %v, want a complaint about the exempt path", err)
	}
}

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
