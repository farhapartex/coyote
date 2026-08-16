package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/view"
)

func TestJSONWritesTypedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := view.JSON(rec, http.StatusCreated, map[string]any{"id": 7, "name": "Kettle"}); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("a JSON body should carry nosniff, got %q", got)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded["name"] != "Kettle" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestJSONErrorAndProblems(t *testing.T) {
	rec := httptest.NewRecorder()
	view.JSONError(rec, http.StatusBadRequest, "")
	if !strings.Contains(rec.Body.String(), "Bad Request") {
		t.Errorf("an empty message should fall back to the status text, got %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	view.JSONProblems(rec, http.StatusUnprocessableEntity, map[string][]string{"email": {"is required"}})
	if !strings.Contains(rec.Body.String(), `"email":["is required"]`) {
		t.Errorf("problems = %s", rec.Body.String())
	}
}

func jsonRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestDecodeReadsAndValidates(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := view.Decode(jsonRequest(`{"name":"Kettle"}`), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "Kettle" {
		t.Errorf("Name = %q", payload.Name)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	err := view.Decode(jsonRequest(`{"name":"Kettle","colour":"red"}`), &payload)
	if err == nil {
		t.Fatal("an unknown field should be an error, not silently dropped")
	}
	if !strings.Contains(err.Error(), "colour") {
		t.Errorf("the error should name the field, got %v", err)
	}
}

func TestDecodeRejectsTrailingContent(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := view.Decode(jsonRequest(`{"name":"a"}{"name":"b"}`), &payload); err == nil {
		t.Error("two values in one body should be refused")
	}
}

func TestDecodeCapsTheBody(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	huge := `{"name":"` + strings.Repeat("x", 500) + `"}`

	err := view.DecodeLimit(jsonRequest(huge), &payload, 64)
	if err == nil {
		t.Fatal("an oversized body should be refused")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %v", err)
	}

	if err := view.DecodeLimit(jsonRequest(`{"name":"ok"}`), &payload, 64); err != nil {
		t.Errorf("a small body should pass: %v", err)
	}
}

func TestDecodeRejectsTheWrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var payload struct {
		Name string `json:"name"`
	}
	if err := view.Decode(req, &payload); err == nil {
		t.Error("a form body should not be decoded as JSON")
	}
}

func TestWantsJSON(t *testing.T) {
	for _, testcase := range []struct {
		accept, contentType string
		want                bool
	}{
		{"application/json", "", true},
		{"", "application/json", true},
		{"text/html,application/json", "", false},
		{"text/html", "", false},
		{"", "", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if testcase.accept != "" {
			req.Header.Set("Accept", testcase.accept)
		}
		if testcase.contentType != "" {
			req.Header.Set("Content-Type", testcase.contentType)
		}
		if got := view.WantsJSON(req); got != testcase.want {
			t.Errorf("accept=%q type=%q: %t, want %t", testcase.accept, testcase.contentType, got, testcase.want)
		}
	}
}
