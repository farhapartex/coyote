package tests

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/store"
	"gorm.io/gorm"
)

type Gadget struct {
	ID        string `gorm:"primaryKey;size:64"`
	Name      string `gorm:"size:100;not null"`
	Stock     int
	Retired   *time.Time
	Published bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func cachedApp(t *testing.T, rows int) (*app.App, model.Store, model.Store, *model.Schema) {
	t.Helper()
	a := newTestApp(t)
	a.RegisterModel(model.Of(Gadget{}))
	syncSchema(t, a)

	records, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := a.Describe(Gadget{})
	if err != nil {
		t.Fatal(err)
	}

	for i := range rows {
		_, err := records.Insert(t.Context(), schema, model.Record{
			"name":      "widget",
			"stock":     int64(i),
			"published": i%2 == 0,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	cached := store.Cached(records, a.Cache(), store.CacheOptions{
		Tables: []string{schema.Table},
		TTL:    time.Minute,
	})
	return a, records, cached, schema
}

func countQueries(t *testing.T, a *app.App) (*atomic.Int64, func()) {
	t.Helper()
	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}
	var queries atomic.Int64
	name := "count-cached-queries"
	if err := handle.Callback().Query().After("gorm:query").Register(name, func(*gorm.DB) {
		queries.Add(1)
	}); err != nil {
		t.Fatal(err)
	}
	return &queries, func() { _ = handle.Callback().Query().Remove(name) }
}

func TestCachedStoreServesAListWithoutQuerying(t *testing.T) {
	a, _, cached, schema := cachedApp(t, 5)

	first, err := cached.List(t.Context(), schema, model.Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Records) != 5 {
		t.Fatalf("got %d records", len(first.Records))
	}

	queries, stop := countQueries(t, a)
	defer stop()

	second, err := cached.List(t.Context(), schema, model.Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Records) != 5 || second.Total != first.Total {
		t.Errorf("cached page = %d records, total %d", len(second.Records), second.Total)
	}
	if queries.Load() != 0 {
		t.Errorf("a cache hit cost %d queries, want 0", queries.Load())
	}
}

func TestCachedStorePreservesRecordTypes(t *testing.T) {
	_, records, cached, schema := cachedApp(t, 1)

	direct, err := records.List(t.Context(), schema, model.Query{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached.List(t.Context(), schema, model.Query{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	fromCache, err := cached.List(t.Context(), schema, model.Query{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}

	want, got := direct.Records[0], fromCache.Records[0]
	if got.String("name") != want.String("name") {
		t.Errorf("name = %q, want %q", got.String("name"), want.String("name"))
	}
	if got.Bool("published") != want.Bool("published") {
		t.Errorf("published = %v, want %v", got.Bool("published"), want.Bool("published"))
	}
	if got.Get("retired") != nil {
		t.Errorf("a null column came back as %#v, want nil", got.Get("retired"))
	}
	if _, ok := got.Get("created_at").(time.Time); !ok {
		t.Errorf("created_at = %#v, want a time.Time", got.Get("created_at"))
	}
	if got.String("stock") != want.String("stock") {
		t.Errorf("stock = %q, want %q; an integer must not become a float", got.String("stock"), want.String("stock"))
	}
}

func TestCachedStoreInvalidatesOnInsert(t *testing.T) {
	_, _, cached, schema := cachedApp(t, 2)

	before, err := cached.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if before != 2 {
		t.Fatalf("count = %d, want 2", before)
	}

	if _, err := cached.Insert(t.Context(), schema, model.Record{"name": "fresh", "stock": int64(9)}); err != nil {
		t.Fatal(err)
	}

	after, err := cached.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if after != 3 {
		t.Errorf("count after an insert = %d, want 3; the write must invalidate", after)
	}
}

func TestCachedStoreInvalidatesOnUpdateAndDelete(t *testing.T) {
	_, _, cached, schema := cachedApp(t, 1)

	page, err := cached.List(t.Context(), schema, model.Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	id := page.Records[0].String("id")

	found, err := cached.Find(t.Context(), schema, id)
	if err != nil {
		t.Fatal(err)
	}
	if found.String("name") != "widget" {
		t.Fatalf("name = %q", found.String("name"))
	}

	if err := cached.Update(t.Context(), schema, id, model.Record{"name": "renamed"}); err != nil {
		t.Fatal(err)
	}
	again, err := cached.Find(t.Context(), schema, id)
	if err != nil {
		t.Fatal(err)
	}
	if again.String("name") != "renamed" {
		t.Errorf("name after update = %q, want %q", again.String("name"), "renamed")
	}

	if err := cached.Delete(t.Context(), schema, id); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Find(t.Context(), schema, id); err == nil {
		t.Error("Find should fail after the row was deleted")
	}
}

func TestCachedStoreLeavesUntrackedTablesAlone(t *testing.T) {
	a, records, _, _ := cachedApp(t, 1)

	users, err := a.Describe(auth.User{})
	if err != nil {
		t.Fatal(err)
	}
	cached := store.Cached(records, a.Cache(), store.CacheOptions{Tables: []string{"gadgets"}})

	if _, err := cached.Count(t.Context(), users, model.Query{}); err != nil {
		t.Fatal(err)
	}

	queries, stop := countQueries(t, a)
	defer stop()

	if _, err := cached.Count(t.Context(), users, model.Query{}); err != nil {
		t.Fatal(err)
	}
	if queries.Load() == 0 {
		t.Error("a table outside CacheOptions.Tables must not be cached")
	}
}

func TestCachedStoreDoesNotCacheUnboundedLists(t *testing.T) {
	a, _, cached, schema := cachedApp(t, 3)

	if _, err := cached.List(t.Context(), schema, model.Query{}); err != nil {
		t.Fatal(err)
	}

	queries, stop := countQueries(t, a)
	defer stop()

	if _, err := cached.List(t.Context(), schema, model.Query{}); err != nil {
		t.Fatal(err)
	}
	if queries.Load() == 0 {
		t.Error("a list with no limit must not be cached")
	}
}

func TestCachedStoreWithTxBypassesTheCache(t *testing.T) {
	a, _, cached, schema := cachedApp(t, 1)

	if _, err := cached.Count(t.Context(), schema, model.Query{}); err != nil {
		t.Fatal(err)
	}

	handle, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}

	err = handle.Transaction(func(tx *gorm.DB) error {
		inside := cached.WithTx(tx)
		if _, err := inside.Insert(t.Context(), schema, model.Record{"name": "inside", "stock": int64(1)}); err != nil {
			return err
		}
		total, err := inside.Count(t.Context(), schema, model.Query{})
		if err != nil {
			return err
		}
		if total != 2 {
			t.Errorf("a read inside the transaction saw %d rows, want 2; it must not be cached", total)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCachedStoreKeepsDifferentQueriesApart(t *testing.T) {
	_, _, cached, schema := cachedApp(t, 4)

	published, err := cached.Count(t.Context(), schema, model.Query{
		Filters: []model.Filter{{Column: "published", Op: model.Eq, Value: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := cached.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if published == all {
		t.Errorf("a filtered count (%d) must not read the unfiltered entry (%d)", published, all)
	}
}

func TestCachedStoreIsOptInOnly(t *testing.T) {
	a, records, _, _ := cachedApp(t, 1)

	if store.Cached(records, a.Cache(), store.CacheOptions{}) != records {
		t.Error("Cached with no tables should hand back the store unchanged")
	}
	if store.Cached(records, nil, store.CacheOptions{Tables: []string{"gadgets"}}) != records {
		t.Error("Cached with no cache should hand back the store unchanged")
	}
}
