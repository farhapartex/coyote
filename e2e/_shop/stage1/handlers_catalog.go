package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

const perPage = 12

type productCard struct {
	Slug         string
	Name         string
	CategoryName string
	CategorySlug string
	Price        string
	PriceCents   int64
	Stock        int
	InStock      bool
	Image        string
	Summary      string
	Description  string
}

type categoryLink struct {
	Name  string
	Slug  string
	Blurb string
}

type filterState struct {
	Category string
	Min      string
	Max      string
	InStock  bool
	Sort     string
}

type sortOption struct {
	Value  string
	Label  string
	Chosen bool
}

var sortLabels = map[string]string{
	"newest":     "Newest first",
	"price-asc":  "Cheapest first",
	"price-desc": "Dearest first",
	"name":       "By name",
}

var sortColumns = map[string]string{
	"newest":     "-created_at",
	"price-asc":  "price_cents",
	"price-desc": "-price_cents",
	"name":       "name",
}

func cardOf(record model.Record) productCard {
	stock := int(recordInt(record, "stock"))
	return productCard{
		Slug:         record.String("slug"),
		Name:         record.String("name"),
		CategoryName: record.String("category_id__label"),
		Price:        money(recordInt(record, "price_cents")),
		PriceCents:   recordInt(record, "price_cents"),
		Stock:        stock,
		InStock:      stock > 0,
		Image:        record.String("image"),
		Summary:      record.String("summary"),
		Description:  record.String("description"),
	}
}

func categoryLinks(ctx context.Context, a *app.App) []categoryLink {
	records, err := a.Store()
	if err != nil {
		return nil
	}
	schema, err := a.Describe(Category{})
	if err != nil {
		return nil
	}
	page, err := records.List(ctx, schema, model.Query{Order: "position,name", Limit: 40})
	if err != nil {
		return nil
	}
	out := make([]categoryLink, 0, len(page.Records))
	for _, record := range page.Records {
		out = append(out, categoryLink{
			Name:  record.String("name"),
			Slug:  record.String("slug"),
			Blurb: record.String("blurb"),
		})
	}
	return out
}

func categoryBySlug(ctx context.Context, a *app.App, slug string) (model.Record, bool) {
	records, err := a.Store()
	if err != nil {
		return nil, false
	}
	schema, err := a.Describe(Category{})
	if err != nil {
		return nil, false
	}
	found, err := records.First(ctx, schema, model.Query{
		Filters: []model.Filter{{Column: "slug", Op: model.Eq, Value: slug}},
	})
	if err != nil {
		return nil, false
	}
	return found, true
}

func landingPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, err := a.Store()
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		schema, err := a.Describe(Product{})
		if err != nil {
			serverError(a, w, r, err)
			return
		}

		featured, err := records.List(r.Context(), schema, model.Query{
			Filters: []model.Filter{
				{Column: "featured", Op: model.Eq, Value: true},
				{Column: "is_active", Op: model.Eq, Value: true},
			},
			With:  []string{"category_id"},
			Order: "-created_at",
			Limit: 8,
		})
		if err != nil {
			serverError(a, w, r, err)
			return
		}

		cards := make([]productCard, 0, len(featured.Records))
		for _, record := range featured.Records {
			cards = append(cards, cardOf(record))
		}

		render(a, w, r, "pages/home.html", app.Data{
			"Title":    "",
			"Nav":      "home",
			"Featured": cards,
		})
	}
}

func productList(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderListing(a, w, r, "Everything in the shop", r.URL.Query().Get("category"))
	}
}

func categoryPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		found, ok := categoryBySlug(r.Context(), a, slug)
		if !ok {
			notFound(a, w, r)
			return
		}
		renderListing(a, w, r, found.String("name"), slug)
	}
}

func renderListing(a *app.App, w http.ResponseWriter, r *http.Request, heading, category string) {
	records, err := a.Store()
	if err != nil {
		serverError(a, w, r, err)
		return
	}
	schema, err := a.Describe(Product{})
	if err != nil {
		serverError(a, w, r, err)
		return
	}

	asked := r.URL.Query()
	state := filterState{
		Category: category,
		Min:      strings.TrimSpace(asked.Get("min")),
		Max:      strings.TrimSpace(asked.Get("max")),
		InStock:  asked.Get("stock") == "in",
		Sort:     asked.Get("sort"),
	}
	if _, known := sortColumns[state.Sort]; !known {
		state.Sort = "newest"
	}

	query := model.Query{
		Filters: []model.Filter{{Column: "is_active", Op: model.Eq, Value: true}},
		With:    []string{"category_id"},
		Order:   sortColumns[state.Sort],
		Limit:   perPage,
	}

	if state.Category != "" {
		found, ok := categoryBySlug(r.Context(), a, state.Category)
		if !ok {
			notFound(a, w, r)
			return
		}
		query.Filters = append(query.Filters, model.Filter{
			Column: "category_id", Op: model.Eq, Value: found.String("id"),
		})
	}
	if cents, err := parseMoney(state.Min); err == nil && state.Min != "" {
		query.Filters = append(query.Filters, model.Filter{Column: "price_cents", Op: model.Gte, Value: cents})
	}
	if cents, err := parseMoney(state.Max); err == nil && state.Max != "" {
		query.Filters = append(query.Filters, model.Filter{Column: "price_cents", Op: model.Lte, Value: cents})
	}
	if state.InStock {
		query.Filters = append(query.Filters, model.Filter{Column: "stock", Op: model.Gt, Value: 0})
	}

	number := 1
	if n, err := strconv.Atoi(asked.Get("page")); err == nil && n > 1 {
		number = n
	}
	query.Offset = (number - 1) * perPage

	result, err := records.List(r.Context(), schema, query)
	if err != nil {
		serverError(a, w, r, err)
		return
	}
	page := view.Paginate(a.Settings.Pagination.Paginator, result.Total, number, perPage)
	if page.Offset != query.Offset {
		query.Offset, query.Limit = page.Offset, page.Limit
		if result, err = records.List(r.Context(), schema, query); err != nil {
			serverError(a, w, r, err)
			return
		}
	}

	cards := make([]productCard, 0, len(result.Records))
	for _, record := range result.Records {
		cards = append(cards, cardOf(record))
	}

	options := make([]sortOption, 0, len(sortColumns))
	for _, value := range []string{"newest", "price-asc", "price-desc", "name"} {
		options = append(options, sortOption{Value: value, Label: sortLabels[value], Chosen: value == state.Sort})
	}

	render(a, w, r, "pages/products.html", app.Data{
		"Title":        heading,
		"Nav":          "products",
		"Heading":      heading,
		"Products":     cards,
		"Total":        result.Total,
		"Page":         page,
		"PageURL":      listingURL(state) + "page=",
		"FilterAction": "/products",
		"Filter":       state,
		"SortOptions":  options,
		"SortLabel":    sortLabels[state.Sort],
	})
}

func listingURL(state filterState) string {
	parts := []string{}
	if state.Category != "" {
		parts = append(parts, "category="+state.Category)
	}
	if state.Min != "" {
		parts = append(parts, "min="+state.Min)
	}
	if state.Max != "" {
		parts = append(parts, "max="+state.Max)
	}
	if state.InStock {
		parts = append(parts, "stock=in")
	}
	if state.Sort != "" {
		parts = append(parts, "sort="+state.Sort)
	}
	return "/products?" + strings.Join(parts, "&") + "&"
}

func productPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, err := a.Store()
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		schema, err := a.Describe(Product{})
		if err != nil {
			serverError(a, w, r, err)
			return
		}

		found, err := records.First(r.Context(), schema, model.Query{
			Filters: []model.Filter{
				{Column: "slug", Op: model.Eq, Value: r.PathValue("slug")},
				{Column: "is_active", Op: model.Eq, Value: true},
			},
			With: []string{"category_id"},
		})
		if err != nil {
			notFound(a, w, r)
			return
		}
		card := cardOf(found)

		related, err := records.List(r.Context(), schema, model.Query{
			Filters: []model.Filter{
				{Column: "category_id", Op: model.Eq, Value: found.String("category_id")},
				{Column: "is_active", Op: model.Eq, Value: true},
				{Column: "slug", Op: model.Ne, Value: card.Slug},
			},
			With:  []string{"category_id"},
			Limit: 4,
		})
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		siblings := make([]productCard, 0, len(related.Records))
		for _, record := range related.Records {
			siblings = append(siblings, cardOf(record))
		}

		if categorySlug, ok := categoryByID(r.Context(), a, found.String("category_id")); ok {
			card.CategorySlug = categorySlug
		}

		render(a, w, r, "pages/product.html", app.Data{
			"Title":   card.Name,
			"Nav":     "products",
			"Product": card,
			"Related": siblings,
		})
	}
}

func categoryByID(ctx context.Context, a *app.App, id string) (string, bool) {
	records, err := a.Store()
	if err != nil {
		return "", false
	}
	schema, err := a.Describe(Category{})
	if err != nil {
		return "", false
	}
	found, err := records.Find(ctx, schema, id)
	if err != nil {
		return "", false
	}
	return found.String("slug"), true
}

func notFound(a *app.App, w http.ResponseWriter, r *http.Request) {
	renderStatus(a, w, r, http.StatusNotFound, "pages/notfound.html", app.Data{"Title": "Not found"})
}

func serverError(a *app.App, w http.ResponseWriter, r *http.Request, err error) {
	a.Logger.Error("shop request failed", "path", r.URL.Path, "error", err)
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
}
