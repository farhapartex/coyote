package admin

import (
	"errors"
	"sort"
	"sync"

	"github.com/farhapartex/coyote/core/model"
)

var ErrUnknownResource = errors.New("coyote/admin: no resource registered for that path")

type managed struct {
	resource  Resource
	schema    *model.Schema
	readOnly  bool
	order     string
	searchOn  []string
	filterOn  []string
	relations []model.Relation
}

func (m managed) Searchable() bool { return len(m.searchOn) > 0 }

func (m managed) Slug() string  { return m.schema.Slug }
func (m managed) Title() string { return m.schema.Plural }
func (m managed) Label() string { return m.schema.Label }

type resourceRegistry struct {
	mu    sync.RWMutex
	items map[string]managed
}

func newResourceRegistry() *resourceRegistry {
	return &resourceRegistry{items: make(map[string]managed)}
}

func (r *resourceRegistry) add(entry managed) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[entry.Slug()] = entry
}

func (r *resourceRegistry) bySlug(slug string) (managed, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.items[slug]
	if !ok {
		return managed{}, ErrUnknownResource
	}
	return entry, nil
}

func (r *resourceRegistry) all() []managed {
	r.mu.RLock()
	out := make([]managed, 0, len(r.items))
	for _, entry := range r.items {
		out = append(out, entry)
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Title() < out[j].Title() })
	return out
}

func describeResource(schema *model.Schema, resource Resource) managed {
	entry := managed{resource: resource, schema: schema}

	if named, ok := resource.(Labelled); ok {
		schema.Label = named.Label()
	}
	if plural, ok := resource.(Pluralised); ok {
		schema.Plural = plural.PluralLabel()
	}
	if routed, ok := resource.(Routed); ok && routed.Slug() != "" {
		schema.Slug = routed.Slug()
	}
	if listed, ok := resource.(Listed); ok {
		schema.SetListColumns(listed.ListColumns())
	}
	if hidden, ok := resource.(Concealed); ok {
		schema.Hide(hidden.HiddenColumns())
	}
	if sorted, ok := resource.(Sorted); ok {
		entry.order = sorted.DefaultOrder()
	}
	if searched, ok := resource.(Searchable); ok {
		entry.searchOn = keepColumns(schema, searched.SearchColumns())
	}
	if filtered, ok := resource.(Filterable); ok {
		entry.filterOn = keepColumns(schema, filtered.FilterColumns())
	}
	entry.relations = schema.Relations
	if guarded, ok := resource.(Guarded); ok {
		entry.readOnly = guarded.ReadOnly()
	}
	return entry
}

func keepColumns(schema *model.Schema, columns []string) []string {
	out := make([]string, 0, len(columns))
	for _, column := range columns {
		if schema.HasColumn(column) {
			out = append(out, column)
		}
	}
	return out
}
