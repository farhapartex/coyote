package tests

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/store"
	"gorm.io/gorm"
)

type Category struct {
	ID   string `gorm:"primaryKey;size:64"`
	Name string `gorm:"size:100;not null"`
}

type Item struct {
	ID         string  `gorm:"primaryKey;size:64"`
	Title      string  `gorm:"size:100;not null"`
	CategoryID *string `gorm:"size:64;index"`
	Category   Category
}

func relatedApp(t *testing.T, items int) (*app.App, model.Store, *model.Schema) {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) { s.Logging.Level = "error" }))
	a.RegisterModel(model.Of(Category{}), model.Of(Item{}))
	syncSchema(t, a)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []Category{{ID: "c1", Name: "Kitchen"}, {ID: "c2", Name: "Garden"}} {
		if err := handle.Create(&category).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := range items {
		category := "c1"
		if i%2 == 1 {
			category = "c2"
		}
		if err := handle.Create(&Item{
			ID: fmt.Sprintf("i-%02d", i), Title: fmt.Sprintf("Item %d", i), CategoryID: &category,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	records, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := a.Describe(Item{})
	if err != nil {
		t.Fatal(err)
	}
	return a, records, schema
}

func TestBelongsToIsDescribed(t *testing.T) {
	_, _, schema := relatedApp(t, 0)

	relation, found := schema.Relation("category_id")
	if !found {
		t.Fatalf("no relation described, got %+v", schema.Relations)
	}
	if relation.Target != "categories" || relation.TargetKey != "id" {
		t.Errorf("relation = %+v", relation)
	}
	if relation.LabelColumn != "name" {
		t.Errorf("LabelColumn = %q, want the target's name column", relation.LabelColumn)
	}
}

func TestLabelsAreResolvedOnlyWhenAsked(t *testing.T) {
	_, records, schema := relatedApp(t, 4)

	plain, err := records.List(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if _, carried := plain.Records[0]["category_id"+store.LabelSuffix]; carried {
		t.Error("a plain list should not resolve relations")
	}

	page, err := records.List(t.Context(), schema, model.Query{With: []string{"category_id"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range page.Records {
		label := fmt.Sprint(record.Get("category_id" + store.LabelSuffix))
		if label != "Kitchen" && label != "Garden" {
			t.Errorf("label = %q for category %q", label, record.String("category_id"))
		}
	}
}

func TestLabelResolutionCostsOneQueryPerRelation(t *testing.T) {
	a, records, schema := relatedApp(t, 20)

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}

	var queries atomic.Int64
	handle.Callback().Query().After("gorm:query").Register("count-queries", func(*gorm.DB) {
		queries.Add(1)
	})
	defer handle.Callback().Query().Remove("count-queries")

	page, err := records.List(t.Context(), schema, model.Query{Limit: 20, With: []string{"category_id"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 20 {
		t.Fatalf("got %d records", len(page.Records))
	}

	if got := queries.Load(); got > 3 {
		t.Errorf("resolving labels for 20 rows took %d queries; it must not be one per row", got)
	}
}

func TestUnknownRelationsAreIgnored(t *testing.T) {
	_, records, schema := relatedApp(t, 2)

	page, err := records.List(t.Context(), schema, model.Query{With: []string{"nonexistent_id"}})
	if err != nil {
		t.Fatalf("an unknown relation should be ignored, got %v", err)
	}
	if len(page.Records) != 2 {
		t.Error("the result should be unaffected")
	}
}

func TestRowsWithoutAKeyLeaveNoLabel(t *testing.T) {
	a, records, schema := relatedApp(t, 1)

	handle, _ := a.DB()
	if err := handle.Create(&Item{ID: "orphan", Title: "Orphan"}).Error; err != nil {
		t.Fatal(err)
	}

	page, err := records.List(t.Context(), schema, model.Query{With: []string{"category_id"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range page.Records {
		if record.String("id") != "orphan" {
			continue
		}
		if label, carried := record["category_id"+store.LabelSuffix]; carried {
			t.Errorf("a row with no key should not get a label, got %v", label)
		}
	}
}
