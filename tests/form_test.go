package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/form"
)

type signup struct {
	Email    string    `form:"email" validate:"required,email"`
	Name     string    `form:"name" validate:"required,max=20"`
	Age      int       `form:"age" validate:"min=18"`
	Rate     float64   `form:"rate"`
	Accepted bool      `form:"accepted" validate:"required"`
	Starts   time.Time `form:"starts"`
	Notes    *string   `form:"notes"`
	Ignored  string    `form:"-"`
	hidden   string
}

func post(values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestBindFillsEveryKind(t *testing.T) {
	var in signup
	problems, err := form.Bind(post(url.Values{
		"email":    {"jane@example.com"},
		"name":     {"  Jane  "},
		"age":      {"30"},
		"rate":     {"2.5"},
		"accepted": {"on"},
		"starts":   {"2026-08-16T09:30"},
		"notes":    {"hello"},
		"-":        {"nope"},
	}), &in)
	if err != nil {
		t.Fatal(err)
	}
	if problems.Any() {
		t.Fatalf("unexpected problems: %v", problems)
	}

	if in.Email != "jane@example.com" || in.Name != "Jane" {
		t.Errorf("strings = %+v", in)
	}
	if in.Age != 30 || in.Rate != 2.5 || !in.Accepted {
		t.Errorf("numbers/bool = %+v", in)
	}
	if in.Starts.Format("2006-01-02 15:04") != "2026-08-16 09:30" {
		t.Errorf("time = %v", in.Starts)
	}
	if in.Notes == nil || *in.Notes != "hello" {
		t.Errorf("pointer = %v", in.Notes)
	}
	if in.Ignored != "" {
		t.Error(`a "-" tag should be skipped`)
	}
}

func TestBindReportsEveryProblemAtOnce(t *testing.T) {
	var in signup
	problems, err := form.Bind(post(url.Values{
		"email": {"not-an-email"},
		"name":  {strings.Repeat("x", 30)},
		"age":   {"12"},
	}), &in)
	if err != nil {
		t.Fatal(err)
	}
	if !problems.Any() {
		t.Fatal("expected problems")
	}

	for _, field := range []string{"email", "name", "age", "accepted"} {
		if !problems.Has(field) {
			t.Errorf("no problem reported for %q: %v", field, problems)
		}
	}
	if got := problems.First("email"); got != "is not a valid email address" {
		t.Errorf("email problem = %q", got)
	}
	if got := strings.Join(problems.Fields(), ","); got != "accepted,age,email,name" {
		t.Errorf("fields should come back sorted, got %q", got)
	}
}

func TestBindReportsUnparseableValues(t *testing.T) {
	var in signup
	problems, err := form.Bind(post(url.Values{
		"email": {"jane@example.com"}, "name": {"Jane"}, "accepted": {"1"},
		"age": {"abc"}, "starts": {"not-a-date"},
	}), &in)
	if err != nil {
		t.Fatal(err)
	}
	if got := problems.First("age"); got != "must be a whole number" {
		t.Errorf("age problem = %q", got)
	}
	if got := problems.First("starts"); got != "is not a valid date" {
		t.Errorf("starts problem = %q", got)
	}
}

func TestValidatorsCoverTheCommonCases(t *testing.T) {
	type sample struct {
		Colour string `form:"colour" validate:"oneof=red|green|blue"`
		Site   string `form:"site" validate:"url"`
		Code   string `form:"code" validate:"len=4,alphanum"`
		Count  string `form:"count" validate:"numeric"`
		Slug   string `form:"slug" validate:"match=^[a-z-]+$"`
	}

	var in sample
	problems, err := form.Values(url.Values{
		"colour": {"purple"},
		"site":   {"not a url"},
		"code":   {"ab!"},
		"count":  {"twelve"},
		"slug":   {"Not A Slug"},
	}, &in)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"colour", "site", "code", "count", "slug"} {
		if !problems.Has(field) {
			t.Errorf("%s should have failed: %v", field, problems)
		}
	}

	problems, err = form.Values(url.Values{
		"colour": {"green"}, "site": {"https://example.com"},
		"code": {"ab12"}, "count": {"12"}, "slug": {"a-slug"},
	}, &in)
	if err != nil {
		t.Fatal(err)
	}
	if problems.Any() {
		t.Errorf("valid input reported problems: %v", problems)
	}
}

func TestCustomRulesCanBeRegistered(t *testing.T) {
	form.Register("shouty", func(value, _ string) error {
		if value != strings.ToUpper(value) {
			return errShouty
		}
		return nil
	})

	type sample struct {
		Word string `form:"word" validate:"shouty"`
	}
	var in sample

	problems, _ := form.Values(url.Values{"word": {"quiet"}}, &in)
	if !problems.Has("word") {
		t.Error("a registered rule should run")
	}
	problems, _ = form.Values(url.Values{"word": {"LOUD"}}, &in)
	if problems.Any() {
		t.Errorf("valid input failed: %v", problems)
	}
}

var errShouty = shoutyError{}

type shoutyError struct{}

func (shoutyError) Error() string { return "must be shouty" }

func TestBindRejectsANonPointer(t *testing.T) {
	var in signup
	if _, err := form.Bind(post(url.Values{}), in); err == nil {
		t.Error("binding to a value should be refused")
	}
	if _, err := form.Bind(post(url.Values{}), nil); err == nil {
		t.Error("binding to nil should be refused")
	}
}

func TestBindQueryReadsTheQueryString(t *testing.T) {
	type filter struct {
		Search string `form:"q"`
		Page   int    `form:"page"`
	}
	req := httptest.NewRequest(http.MethodGet, "/items?q=kettle&page=3", nil)

	var in filter
	if _, err := form.BindQuery(req, &in); err != nil {
		t.Fatal(err)
	}
	if in.Search != "kettle" || in.Page != 3 {
		t.Errorf("filter = %+v", in)
	}
}

func TestRecordBinderIsSharedWithTheAdmin(t *testing.T) {
	a := migratedApp(t, Product{})
	schema, err := a.Describe(Product{})
	if err != nil {
		t.Fatal(err)
	}

	bound := form.Record(url.Values{
		"name":  {"Kettle"},
		"sku":   {"KTL-1"},
		"price": {"19.50"},
		"stock": {"4"},
	}, schema, true)
	if !bound.Valid() {
		t.Fatalf("problems: %v", bound.Problems)
	}
	if bound.Record.String("name") != "Kettle" {
		t.Errorf("record = %v", bound.Record)
	}
	if got, ok := bound.Record["price"].(float64); !ok || got != 19.5 {
		t.Errorf("price = %#v", bound.Record["price"])
	}

	bad := form.Record(url.Values{"name": {"Kettle"}, "price": {"free"}}, schema, true)
	if bad.Valid() {
		t.Fatal("expected problems")
	}
	if !bad.Problems.Has("price") {
		t.Errorf("problems = %v", bad.Problems)
	}
	if got := bad.Errors()["price"]; got == "" {
		t.Error("Errors() should flatten problems for the admin templates")
	}
	if !bad.Problems.Has("sku") {
		t.Error("a required column left blank should be reported")
	}
}

func TestNarrowNumbersRefuseWhatTheyCannotHold(t *testing.T) {
	type stock struct {
		Small  int8    `form:"small"`
		Tiny   uint8   `form:"tiny"`
		Single float32 `form:"single"`
	}

	var in stock
	problems, err := form.Values(url.Values{
		"small":  {"200"},
		"tiny":   {"256"},
		"single": {"1e40"},
	}, &in)
	if err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{"small", "tiny", "single"} {
		if problems.First(field) != "is out of range" {
			t.Errorf("%s problem = %q, want it refused rather than truncated", field, problems.First(field))
		}
	}
	if in.Small != 0 || in.Tiny != 0 || in.Single != 0 {
		t.Errorf("values = %d, %d, %v; nothing out of range should have been assigned",
			in.Small, in.Tiny, in.Single)
	}
}

func TestARangeErrorIsNotConfusedWithBadDigits(t *testing.T) {
	type counts struct {
		N int8 `form:"n"`
	}

	var in counts
	problems, err := form.Values(url.Values{"n": {"banana"}}, &in)
	if err != nil {
		t.Fatal(err)
	}
	if got := problems.First("n"); got != "must be a whole number" {
		t.Errorf("problem = %q, want the malformed message rather than the range one", got)
	}
}

func TestValidationSeesTheValueThatWasStored(t *testing.T) {
	type order struct {
		Quantity int8 `form:"quantity" validate:"min=1"`
	}

	var in order
	problems, err := form.Values(url.Values{"quantity": {"200"}}, &in)
	if err != nil {
		t.Fatal(err)
	}
	if !problems.Any() {
		t.Fatal("200 does not fit an int8; it must not pass as a valid quantity")
	}
	if in.Quantity < 0 {
		t.Errorf("Quantity = %d; a wrapped negative would pass min=1 while meaning the opposite", in.Quantity)
	}
}
