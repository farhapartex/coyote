package main

import (
	"net/http"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
)

type orderResource struct {
	a *app.App
}

func (orderResource) Entity() any { return Order{} }

func (orderResource) Label() string { return "Order" }

func (orderResource) ListColumns() []string {
	return []string{"reference", "state", "email", "total_cents", "placed_at"}
}

func (orderResource) DefaultOrder() string { return "-placed_at" }

func (orderResource) SearchColumns() []string { return []string{"reference", "email", "ship_to"} }

func (orderResource) FilterColumns() []string { return nil }

func (r orderResource) Actions() []admin.Action {
	return []admin.Action{{
		Name:       "advance",
		Label:      "Advance delivery",
		Confirm:    "Move the selected orders to their next delivery state?",
		Permission: "update",
		Run: func(request *http.Request, ids []string) (int, error) {
			return advanceSelected(r.a, request, ids)
		},
	}}
}

type orderLineResource struct{}

func (orderLineResource) Entity() any { return OrderLine{} }

func (orderLineResource) Label() string { return "Order line" }

func (orderLineResource) ListColumns() []string {
	return []string{"order_id", "title", "quantity", "line_cents"}
}

func (orderLineResource) ReadOnly() bool { return true }

func (orderLineResource) DefaultOrder() string { return "-id" }

type shipmentEventResource struct{}

func (shipmentEventResource) Entity() any { return ShipmentEvent{} }

func (shipmentEventResource) Label() string { return "Shipment event" }

func (shipmentEventResource) PluralLabel() string { return "Shipment events" }

func (shipmentEventResource) ListColumns() []string {
	return []string{"order_id", "state", "note", "happened_at"}
}

func (shipmentEventResource) DefaultOrder() string { return "-happened_at" }

type couponResource struct{}

func (couponResource) Entity() any { return Coupon{} }

func (couponResource) Label() string { return "Coupon" }

func (couponResource) ListColumns() []string { return []string{"code", "percent_off", "is_active"} }

func (couponResource) DefaultOrder() string { return "code" }

func (couponResource) FilterColumns() []string { return []string{"is_active"} }

type stockMovementResource struct{}

func (stockMovementResource) Entity() any { return StockMovement{} }

func (stockMovementResource) Label() string { return "Stock movement" }

func (stockMovementResource) PluralLabel() string { return "Stock movements" }

func (stockMovementResource) ListColumns() []string {
	return []string{"product_id", "delta", "reason", "created_at"}
}

func (stockMovementResource) ReadOnly() bool { return true }

func (stockMovementResource) DefaultOrder() string { return "-created_at" }

type addressResource struct{}

func (addressResource) Entity() any { return Address{} }

func (addressResource) Label() string { return "Address" }

func (addressResource) PluralLabel() string { return "Addresses" }

func (addressResource) ListColumns() []string { return []string{"label", "line1", "city", "postcode"} }

func (addressResource) SearchColumns() []string { return []string{"line1", "city", "postcode"} }
