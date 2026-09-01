package tests

import (
	"errors"
	"fmt"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
)

type Widget2 struct {
	ID      string `gorm:"primaryKey;size:64"`
	Name    string `gorm:"size:100;not null"`
	Price   float64
	Stock   int
	Retired bool
	Notes   string `gorm:"size:2000"`
}

func queryApp(t *testing.T, rows int) (*app.App, model.Store, *model.Schema) {
	t.Helper()
	a := migratedApp(t, Widget2{})

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if err := handle.Create(&Widget2{
			ID:      fmt.Sprintf("w-%02d", i),
			Name:    fmt.Sprintf("Widget %02d", i),
			Price:   float64(i) * 10,
			Stock:   i,
			Retired: i%2 == 0,
			Notes:   "a long note",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	records, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := a.Describe(Widget2{})
	if err != nil {
		t.Fatal(err)
	}
	return a, records, schema
}

func TestSortIsValidatedAgainstTheSchema(t *testing.T) {
	_, records, schema := queryApp(t, 5)

	page, err := records.List(t.Context(), schema, model.Query{Sort: "-name"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Records[0].String("name") != "Widget 04" {
		t.Errorf("descending sort failed: %q", page.Records[0].String("name"))
	}

	page, err = records.List(t.Context(), schema, model.Query{Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Records[0].String("name") != "Widget 00" {
		t.Errorf("ascending sort failed: %q", page.Records[0].String("name"))
	}
}

func TestAnInjectedSortIsIgnored(t *testing.T) {
	_, records, schema := queryApp(t, 3)

	for _, hostile := range []string{
		"name; drop table widget2s",
		"(select 1)",
		"name asc, price desc",
		"nonexistent",
		"1",
	} {
		page, err := records.List(t.Context(), schema, model.Query{Sort: hostile})
		if err != nil {
			t.Errorf("Sort %q returned an error instead of being ignored: %v", hostile, err)
			continue
		}
		if len(page.Records) != 3 {
			t.Errorf("Sort %q disturbed the result set", hostile)
		}
	}

	if _, _, ok := schema.SortColumn("name; drop table x"); ok {
		t.Error("SortColumn must refuse anything that is not a plain column")
	}
	if column, direction, ok := schema.SortColumn("price desc"); !ok || column != "price" || direction != "desc" {
		t.Errorf("SortColumn(price desc) = %q/%q/%t", column, direction, ok)
	}
}

func TestFiltersNarrowTheResultAndTheCount(t *testing.T) {
	_, records, schema := queryApp(t, 10)

	query := model.Query{Filters: []model.Filter{{Column: "retired", Op: model.Eq, Value: true}}}
	page, err := records.List(t.Context(), schema, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 5 {
		t.Errorf("returned %d rows, want 5", len(page.Records))
	}
	if page.Total != 5 {
		t.Errorf("Total = %d; the count must use the same conditions as the select", page.Total)
	}
}

func TestEveryOperator(t *testing.T) {
	_, records, schema := queryApp(t, 10)

	for _, testcase := range []struct {
		name  string
		query model.Query
		want  int64
	}{
		{"eq", q("stock", model.Eq, 3), 1},
		{"ne", q("stock", model.Ne, 3), 9},
		{"lt", q("stock", model.Lt, 3), 3},
		{"lte", q("stock", model.Lte, 3), 4},
		{"gt", q("stock", model.Gt, 7), 2},
		{"gte", q("stock", model.Gte, 7), 3},
		{"like", q("name", model.Like, "Widget 0%"), 10},
		{"in", q("stock", model.In, []any{1, 2, 3}), 3},
		{"in empty", q("stock", model.In, []any{}), 0},
		{"notnull", q("name", model.NotNull, nil), 10},
	} {
		total, err := records.Count(t.Context(), schema, testcase.query)
		if err != nil {
			t.Errorf("%s: %v", testcase.name, err)
			continue
		}
		if total != testcase.want {
			t.Errorf("%s: counted %d, want %d", testcase.name, total, testcase.want)
		}
	}
}

func q(column string, op model.Op, value any) model.Query {
	return model.Query{Filters: []model.Filter{{Column: column, Op: op, Value: value}}}
}

func TestAnUnknownFilterColumnIsRefused(t *testing.T) {
	_, records, schema := queryApp(t, 3)

	_, err := records.List(t.Context(), schema, q("password_or_injection", model.Eq, "x"))
	if !errors.Is(err, store.ErrBadFilter) {
		t.Errorf("error = %v, want ErrBadFilter", err)
	}

	_, err = records.List(t.Context(), schema, q("name", "unknown-op", "x"))
	if !errors.Is(err, store.ErrBadFilter) {
		t.Errorf("an unknown operator should be refused, got %v", err)
	}

	_, err = records.List(t.Context(), schema, q("stock", model.In, "not-a-slice"))
	if !errors.Is(err, store.ErrBadFilter) {
		t.Errorf("In needs a slice, got %v", err)
	}
}

func TestSelectNarrowsTheColumnsFetched(t *testing.T) {
	_, records, schema := queryApp(t, 2)

	page, err := records.List(t.Context(), schema, model.Query{Select: []string{"name"}})
	if err != nil {
		t.Fatal(err)
	}
	record := page.Records[0]
	if record.String("name") == "" {
		t.Error("the selected column should be present")
	}
	if _, carried := record["notes"]; carried {
		t.Error("an unselected column should not be fetched")
	}
	if record.String("id") == "" {
		t.Error("the key is always fetched, so rows stay addressable")
	}

	page, err = records.List(t.Context(), schema, model.Query{Select: []string{"name", "nonexistent"}})
	if err != nil {
		t.Fatalf("an unknown column should be dropped, not fail: %v", err)
	}
	if _, carried := page.Records[0]["nonexistent"]; carried {
		t.Error("an unknown column must not reach the query")
	}
}

func TestExistsAndFirst(t *testing.T) {
	_, records, schema := queryApp(t, 5)

	found, err := records.Exists(t.Context(), schema, q("name", model.Eq, "Widget 02"))
	if err != nil || !found {
		t.Errorf("Exists = %t, %v", found, err)
	}
	found, err = records.Exists(t.Context(), schema, q("name", model.Eq, "absent"))
	if err != nil || found {
		t.Errorf("Exists on a missing row = %t, %v", found, err)
	}

	record, err := records.First(t.Context(), schema, model.Query{Sort: "-stock"})
	if err != nil {
		t.Fatal(err)
	}
	if record.String("name") != "Widget 04" {
		t.Errorf("First = %q", record.String("name"))
	}

	if _, err := records.First(t.Context(), schema, q("name", model.Eq, "absent")); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("First on an empty result = %v, want ErrNotFound", err)
	}
}

func TestOrderIsValidatedAgainstTheSchemaToo(t *testing.T) {
	_, records, schema := queryApp(t, 4)

	page, err := records.List(t.Context(), schema, model.Query{Order: "name desc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 4 || page.Records[0].String("name") != "Widget 03" {
		t.Errorf("first row = %q, want the last widget first", page.Records[0].String("name"))
	}

	page, err = records.List(t.Context(), schema, model.Query{Order: "-stock, name asc"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Records[0].String("name") != "Widget 03" {
		t.Errorf("first row = %q, want the highest stock first", page.Records[0].String("name"))
	}
}

func TestOrderCannotCarrySQL(t *testing.T) {
	_, records, schema := queryApp(t, 3)

	hostile := []string{
		"name; DROP TABLE widget2s",
		"(SELECT 1)",
		"name COLLATE NOCASE",
		"1",
		"CASE WHEN 1 THEN name END",
		"nosuchcolumn",
		"name asc; DELETE FROM widget2s",
	}
	for _, order := range hostile {
		page, err := records.List(t.Context(), schema, model.Query{Order: order})
		if err != nil {
			t.Errorf("Order %q returned an error rather than being ignored: %v", order, err)
			continue
		}
		if len(page.Records) != 3 {
			t.Errorf("Order %q left %d of 3 rows", order, len(page.Records))
		}
	}

	total, err := records.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("%d rows survive, want 3; something in that list executed", total)
	}
}
