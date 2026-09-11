package main

const (
	OrderPlaced    = "placed"
	OrderPacking   = "packing"
	OrderShipped   = "shipped"
	OrderDelivered = "delivered"
	OrderCancelled = "cancelled"
)

var orderFlow = []string{OrderPlaced, OrderPacking, OrderShipped, OrderDelivered}

var orderStateLabels = map[string]string{
	OrderPlaced:    "Order placed",
	OrderPacking:   "Being packed",
	OrderShipped:   "On its way",
	OrderDelivered: "Delivered",
	OrderCancelled: "Cancelled",
}

func nextOrderState(current string) (string, bool) {
	for index, state := range orderFlow {
		if state != current {
			continue
		}
		if index+1 >= len(orderFlow) {
			return "", false
		}
		return orderFlow[index+1], true
	}
	return "", false
}

func orderStateLabel(state string) string {
	if label, found := orderStateLabels[state]; found {
		return label
	}
	return state
}
