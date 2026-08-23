package main

import (
	"context"
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

type productRow struct {
	Name  string
	SKU   string
	Price float64
	Stock int
}

type productReport struct {
	Rows  []productRow
	Total int64
	Built time.Time
}

func reportHandler(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := cache.Remember(r.Context(), a.Cache(), "reports:products", time.Minute,
			func(ctx context.Context) (productReport, error) {
				return buildProductReport(ctx, a)
			})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		a.Render(w, r, "pages/reports.html", view.Data{
			"Title":  "Product report",
			"Report": report,
			"Age":    time.Since(report.Built).Round(time.Second).String(),
		})
	}
}

func buildProductReport(ctx context.Context, a *app.App) (productReport, error) {
	records, err := a.Store()
	if err != nil {
		return productReport{}, err
	}
	schema, err := a.Describe(Product{})
	if err != nil {
		return productReport{}, err
	}

	page, err := records.List(ctx, schema, model.Query{Limit: 50})
	if err != nil {
		return productReport{}, err
	}

	report := productReport{Total: page.Total, Built: time.Now()}
	for _, record := range page.Records {
		row := productRow{
			Name: record.String("name"),
			SKU:  record.String("sku"),
		}
		if price, ok := record.Get("price").(float64); ok {
			row.Price = price
		}
		if stock, ok := record.Get("stock").(int64); ok {
			row.Stock = int(stock)
		}
		report.Rows = append(report.Rows, row)
	}
	return report, nil
}
