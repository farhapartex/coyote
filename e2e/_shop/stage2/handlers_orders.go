package main

import (
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
)

type orderRow struct {
	Reference  string
	State      string
	StateLabel string
	Total      string
	Placed     time.Time
	Lines      int
}

type orderDetail struct {
	Reference  string
	State      string
	StateLabel string
	Email      string
	ShipTo     string
	Coupon     string
	Goods      string
	Discount   string
	Shipping   string
	Total      string
	Placed     time.Time
	Lines      []orderLineRow
	Events     []orderEventRow
}

type orderLineRow struct {
	Title    string
	Quantity int
	Unit     string
	Line     string
}

type orderEventRow struct {
	State      string
	StateLabel string
	Note       string
	When       time.Time
	Done       bool
}

func orderList(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := a.Auth.CurrentUser(r)
		records, err := a.Store()
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		schema, err := a.Describe(Order{})
		if err != nil {
			serverError(a, w, r, err)
			return
		}

		page, err := records.List(r.Context(), schema, model.Query{
			Filters: []model.Filter{{Column: "user_id", Op: model.Eq, Value: user.ID}},
			Order:   "-placed_at",
			Limit:   50,
		})
		if err != nil {
			serverError(a, w, r, err)
			return
		}

		rows := make([]orderRow, 0, len(page.Records))
		for _, record := range page.Records {
			state := record.String("state")
			rows = append(rows, orderRow{
				Reference:  record.String("reference"),
				State:      state,
				StateLabel: orderStateLabel(state),
				Total:      money(recordInt(record, "total_cents")),
				Placed:     timeOf(record, "placed_at"),
			})
		}

		render(a, w, r, "pages/orders.html", app.Data{
			"Title":  "Your orders",
			"Nav":    "orders",
			"Orders": rows,
		})
	}
}

func orderPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := a.Auth.CurrentUser(r)
		records, err := a.Store()
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		orderSchema, err := a.Describe(Order{})
		if err != nil {
			serverError(a, w, r, err)
			return
		}

		found, err := records.First(r.Context(), orderSchema, model.Query{
			Filters: []model.Filter{
				{Column: "reference", Op: model.Eq, Value: r.PathValue("reference")},
				{Column: "user_id", Op: model.Eq, Value: user.ID},
			},
		})
		if err != nil {
			notFound(a, w, r)
			return
		}

		state := found.String("state")
		detail := orderDetail{
			Reference:  found.String("reference"),
			State:      state,
			StateLabel: orderStateLabel(state),
			Email:      found.String("email"),
			ShipTo:     found.String("ship_to"),
			Coupon:     found.String("coupon_code"),
			Goods:      money(recordInt(found, "goods_cents")),
			Discount:   money(recordInt(found, "discount_cents")),
			Shipping:   money(recordInt(found, "shipping_cents")),
			Total:      money(recordInt(found, "total_cents")),
			Placed:     timeOf(found, "placed_at"),
		}

		lineSchema, err := a.Describe(OrderLine{})
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		lines, err := records.List(r.Context(), lineSchema, model.Query{
			Filters: []model.Filter{{Column: "order_id", Op: model.Eq, Value: found.String("id")}},
			Order:   "title",
			Limit:   200,
		})
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		for _, record := range lines.Records {
			detail.Lines = append(detail.Lines, orderLineRow{
				Title:    record.String("title"),
				Quantity: int(recordInt(record, "quantity")),
				Unit:     money(recordInt(record, "unit_cents")),
				Line:     money(recordInt(record, "line_cents")),
			})
		}

		eventSchema, err := a.Describe(ShipmentEvent{})
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		events, err := records.List(r.Context(), eventSchema, model.Query{
			Filters: []model.Filter{{Column: "order_id", Op: model.Eq, Value: found.String("id")}},
			Order:   "happened_at",
			Limit:   50,
		})
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		seen := map[string]bool{}
		for _, record := range events.Records {
			eventState := record.String("state")
			seen[eventState] = true
			detail.Events = append(detail.Events, orderEventRow{
				State:      eventState,
				StateLabel: orderStateLabel(eventState),
				Note:       record.String("note"),
				When:       timeOf(record, "happened_at"),
				Done:       true,
			})
		}
		for _, upcoming := range orderFlow {
			if seen[upcoming] {
				continue
			}
			detail.Events = append(detail.Events, orderEventRow{
				State:      upcoming,
				StateLabel: orderStateLabel(upcoming),
				Done:       false,
			})
		}

		render(a, w, r, "pages/order.html", app.Data{
			"Title": "Order " + detail.Reference,
			"Nav":   "orders",
			"Order": detail,
		})
	}
}

func timeOf(record model.Record, column string) time.Time {
	if stamp, ok := record.Get(column).(time.Time); ok {
		return stamp
	}
	return time.Time{}
}
