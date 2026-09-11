package main

import (
	"net/http"
	"sort"

	"github.com/farhapartex/coyote/core/session"
)

const cartKey = "shop_cart"

type cartLine struct {
	Slug     string
	Name     string
	Unit     string
	Quantity int
	Stock    int
	Subtotal string
	Cents    int64
}

func cartOf(r *http.Request) map[string]int {
	sess := session.FromRequest(r)
	if sess == nil {
		return map[string]int{}
	}
	stored, ok := sess.Get(cartKey).(map[string]int)
	if !ok || stored == nil {
		return map[string]int{}
	}
	out := make(map[string]int, len(stored))
	for slug, quantity := range stored {
		out[slug] = quantity
	}
	return out
}

func saveCart(r *http.Request, cart map[string]int) {
	sess := session.FromRequest(r)
	if sess == nil {
		return
	}
	if len(cart) == 0 {
		sess.Delete(cartKey)
		return
	}
	sess.Set(cartKey, cart)
}

func cartCount(r *http.Request) int {
	total := 0
	for _, quantity := range cartOf(r) {
		total += quantity
	}
	return total
}

func cartSlugs(cart map[string]int) []string {
	out := make([]string, 0, len(cart))
	for slug := range cart {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}
