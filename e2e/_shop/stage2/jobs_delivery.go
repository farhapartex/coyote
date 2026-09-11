package main

import (
	"context"
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/lib/id"
)

const JobAdvanceOrder = "shop.order.advance"

type advanceArgs struct {
	Reference string `json:"reference"`
}

var advanceNotes = map[string]string{
	OrderPacking:   "Picked from the shelf and boxed.",
	OrderShipped:   "Collected by the courier.",
	OrderDelivered: "Left with the customer.",
}

func registerDeliveryJob(a *app.App) {
	jobs.Handle(JobAdvanceOrder, func(ctx context.Context, args advanceArgs) error {
		return advanceOrder(ctx, a, args.Reference)
	})
}

func advanceSelected(a *app.App, r *http.Request, ids []string) (int, error) {
	records, err := a.Store()
	if err != nil {
		return 0, err
	}
	schema, err := a.Describe(Order{})
	if err != nil {
		return 0, err
	}

	moved := 0
	for _, orderID := range ids {
		found, err := records.Find(r.Context(), schema, orderID)
		if err != nil {
			return moved, err
		}
		if err := advanceOrder(r.Context(), a, found.String("reference")); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

func advanceOrder(ctx context.Context, a *app.App, reference string) error {
	records, err := a.Store()
	if err != nil {
		return err
	}
	orderSchema, err := a.Describe(Order{})
	if err != nil {
		return err
	}
	found, err := records.First(ctx, orderSchema, model.Query{
		Filters: []model.Filter{{Column: "reference", Op: model.Eq, Value: reference}},
	})
	if err != nil {
		return err
	}

	current := found.String("state")
	next, ok := nextOrderState(current)
	if !ok {
		return nil
	}

	eventSchema, err := a.Describe(ShipmentEvent{})
	if err != nil {
		return err
	}

	err = a.Transaction(ctx, func(tx *app.Tx) error {
		if err := tx.Store().Update(ctx, orderSchema, found.String("id"), model.Record{
			"state": next,
		}); err != nil {
			return err
		}
		if _, err := tx.Store().Insert(ctx, eventSchema, model.Record{
			"id":          id.MustNew(),
			"order_id":    found.String("id"),
			"state":       next,
			"note":        advanceNotes[next],
			"happened_at": time.Now().UTC(),
		}); err != nil {
			return err
		}
		if _, more := nextOrderState(next); !more {
			return nil
		}
		_, err := jobs.Enqueue(ctx, tx.Queue(), JobAdvanceOrder, advanceArgs{Reference: reference},
			jobs.Options{Delay: 2 * time.Second})
		return err
	})
	return err
}
