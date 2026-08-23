package store

import (
	"context"
	"strconv"
	"time"

	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
)

const (
	generationPrefix = "gen:"
	rowPrefix        = "row:"
	queryPrefix      = "query:"
	countPrefix      = "count:"
)

type CacheOptions struct {
	Tables []string
	TTL    time.Duration
}

type cachedStore struct {
	inner  model.Store
	cache  cache.Cache
	ttl    time.Duration
	tables map[string]bool
}

func Cached(inner model.Store, entries cache.Cache, opts CacheOptions) model.Store {
	if inner == nil || entries == nil || len(opts.Tables) == 0 {
		return inner
	}
	tables := make(map[string]bool, len(opts.Tables))
	for _, table := range opts.Tables {
		tables[table] = true
	}
	if opts.TTL <= 0 {
		opts.TTL = cache.DefaultTTL
	}
	return &cachedStore{inner: inner, cache: entries, ttl: opts.TTL, tables: tables}
}

func (s *cachedStore) WithTx(tx *gorm.DB) model.Store { return s.inner.WithTx(tx) }

func (s *cachedStore) Options(ctx context.Context, relation model.Relation, limit int) ([]model.Record, error) {
	optioner, ok := s.inner.(model.Optioner)
	if !ok {
		return nil, nil
	}
	return optioner.Options(ctx, relation, limit)
}

func (s *cachedStore) caches(schema *model.Schema) bool { return s.tables[schema.Table] }

func (s *cachedStore) generation(ctx context.Context, table string) string {
	raw, found, err := s.cache.Get(ctx, generationPrefix+table)
	if err != nil || !found {
		return "0"
	}
	return string(raw)
}

func (s *cachedStore) invalidate(ctx context.Context, table string) {
	if _, err := s.counter(ctx, table); err == nil {
		return
	}
	if namespacer, ok := s.cache.(cache.Namespacer); ok {
		for _, prefix := range []string{rowPrefix, queryPrefix, countPrefix} {
			_ = namespacer.ClearPrefix(ctx, prefix+table+":")
		}
		return
	}
	_ = s.cache.Clear(ctx)
}

func (s *cachedStore) counter(ctx context.Context, table string) (int64, error) {
	counter, ok := s.cache.(cache.Counter)
	if !ok {
		return 0, cache.ErrUnsupported
	}
	return counter.Incr(ctx, generationPrefix+table, 1, cache.Forever)
}

func (s *cachedStore) key(ctx context.Context, prefix string, schema *model.Schema, suffix string) string {
	return prefix + schema.Table + ":" + s.generation(ctx, schema.Table) + ":" + suffix
}

func (s *cachedStore) List(ctx context.Context, schema *model.Schema, query model.Query) (model.Page, error) {
	if !s.caches(schema) || query.Limit <= 0 {
		return s.inner.List(ctx, schema, query)
	}

	key := s.key(ctx, queryPrefix, schema, queryDigest(query))
	if raw, found, err := s.cache.Get(ctx, key); err == nil && found {
		if page, err := decodePage(raw); err == nil {
			return page, nil
		}
	}

	page, err := s.inner.List(ctx, schema, query)
	if err != nil {
		return page, err
	}
	if raw, err := encodePage(page); err == nil {
		_ = s.cache.Set(ctx, key, raw, s.ttl)
	}
	return page, nil
}

func (s *cachedStore) Find(ctx context.Context, schema *model.Schema, id string) (model.Record, error) {
	if !s.caches(schema) {
		return s.inner.Find(ctx, schema, id)
	}

	key := s.key(ctx, rowPrefix, schema, id)
	if raw, found, err := s.cache.Get(ctx, key); err == nil && found {
		if page, err := decodePage(raw); err == nil && len(page.Records) == 1 {
			return page.Records[0], nil
		}
	}

	record, err := s.inner.Find(ctx, schema, id)
	if err != nil {
		return record, err
	}
	if raw, err := encodePage(model.Page{Records: []model.Record{record}, Total: 1}); err == nil {
		_ = s.cache.Set(ctx, key, raw, s.ttl)
	}
	return record, nil
}

func (s *cachedStore) First(ctx context.Context, schema *model.Schema, query model.Query) (model.Record, error) {
	if !s.caches(schema) {
		return s.inner.First(ctx, schema, query)
	}

	key := s.key(ctx, queryPrefix, schema, "first:"+queryDigest(query))
	if raw, found, err := s.cache.Get(ctx, key); err == nil && found {
		if page, err := decodePage(raw); err == nil && len(page.Records) == 1 {
			return page.Records[0], nil
		}
	}

	record, err := s.inner.First(ctx, schema, query)
	if err != nil {
		return record, err
	}
	if raw, err := encodePage(model.Page{Records: []model.Record{record}, Total: 1}); err == nil {
		_ = s.cache.Set(ctx, key, raw, s.ttl)
	}
	return record, nil
}

func (s *cachedStore) Count(ctx context.Context, schema *model.Schema, query model.Query) (int64, error) {
	if !s.caches(schema) {
		return s.inner.Count(ctx, schema, query)
	}

	key := s.key(ctx, countPrefix, schema, queryDigest(query))
	if raw, found, err := s.cache.Get(ctx, key); err == nil && found {
		if total, err := strconv.ParseInt(string(raw), 10, 64); err == nil {
			return total, nil
		}
	}

	total, err := s.inner.Count(ctx, schema, query)
	if err != nil {
		return total, err
	}
	_ = s.cache.Set(ctx, key, strconv.AppendInt(nil, total, 10), s.ttl)
	return total, nil
}

func (s *cachedStore) Exists(ctx context.Context, schema *model.Schema, query model.Query) (bool, error) {
	if !s.caches(schema) {
		return s.inner.Exists(ctx, schema, query)
	}
	total, err := s.Count(ctx, schema, query)
	return total > 0, err
}

func (s *cachedStore) Insert(ctx context.Context, schema *model.Schema, record model.Record) (string, error) {
	id, err := s.inner.Insert(ctx, schema, record)
	if err == nil && s.caches(schema) {
		s.invalidate(ctx, schema.Table)
	}
	return id, err
}

func (s *cachedStore) Update(ctx context.Context, schema *model.Schema, id string, record model.Record) error {
	err := s.inner.Update(ctx, schema, id, record)
	if err == nil && s.caches(schema) {
		s.invalidate(ctx, schema.Table)
	}
	return err
}

func (s *cachedStore) Delete(ctx context.Context, schema *model.Schema, id string) error {
	err := s.inner.Delete(ctx, schema, id)
	if err == nil && s.caches(schema) {
		s.invalidate(ctx, schema.Table)
	}
	return err
}
