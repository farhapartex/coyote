package main

import (
	"net/http"
	"strconv"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

const shippingCents = 495

type cartView struct {
	Lines      []cartLine
	GoodsCents int64
	Goods      string
	Shipping   string
	Discount   string
	Total      string
	TotalCents int64
	Count      int
	Coupon     string
	PercentOff int
}

func loadCart(a *app.App, r *http.Request) (cartView, error) {
	out := cartView{Lines: []cartLine{}}
	cart := cartOf(r)
	if len(cart) == 0 {
		out.Shipping = money(0)
		out.Goods = money(0)
		out.Total = money(0)
		return out, nil
	}

	records, err := a.Store()
	if err != nil {
		return out, err
	}
	schema, err := a.Describe(Product{})
	if err != nil {
		return out, err
	}

	for _, slug := range cartSlugs(cart) {
		found, err := records.First(r.Context(), schema, model.Query{
			Filters: []model.Filter{{Column: "slug", Op: model.Eq, Value: slug}},
		})
		if err != nil {
			continue
		}
		quantity := cart[slug]
		unit := recordInt(found, "price_cents")
		line := cartLine{
			Slug:     slug,
			Name:     found.String("name"),
			Unit:     money(unit),
			Quantity: quantity,
			Stock:    int(recordInt(found, "stock")),
			Cents:    unit * int64(quantity),
		}
		line.Subtotal = money(line.Cents)
		out.Lines = append(out.Lines, line)
		out.GoodsCents += line.Cents
		out.Count += quantity
	}

	out.Coupon, out.PercentOff = couponInSession(a, r)
	discount := out.GoodsCents * int64(out.PercentOff) / 100
	shipping := int64(shippingCents)
	if out.GoodsCents == 0 {
		shipping = 0
	}

	out.Goods = money(out.GoodsCents)
	out.Discount = money(discount)
	out.Shipping = money(shipping)
	out.TotalCents = out.GoodsCents - discount + shipping
	out.Total = money(out.TotalCents)
	return out, nil
}

func cartPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		basket, err := loadCart(a, r)
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		render(a, w, r, "pages/cart.html", app.Data{
			"Title": "Your cart",
			"Nav":   "cart",
			"Cart":  basket,
		})
	}
}

func cartAdd(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		slug := r.PostForm.Get("product")
		quantity, err := strconv.Atoi(r.PostForm.Get("quantity"))
		if err != nil || quantity < 1 {
			quantity = 1
		}

		stock, name, ok := productStock(a, r, slug)
		if !ok {
			view.Error(r, "That product is no longer available.")
			view.Redirect(w, r, "/products")
			return
		}
		if stock < 1 {
			view.Error(r, name+" is out of stock.")
			view.Redirect(w, r, "/products/"+slug)
			return
		}

		cart := cartOf(r)
		cart[slug] += quantity
		if cart[slug] > stock {
			cart[slug] = stock
			view.Warning(r, "Only "+strconv.Itoa(stock)+" of "+name+" are in stock.")
		} else {
			view.Success(r, name+" added to your cart.")
		}
		saveCart(r, cart)
		view.Redirect(w, r, "/cart")
	}
}

func cartUpdate(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		cart := cartOf(r)
		for slug := range cart {
			raw := r.PostForm.Get("quantity_" + slug)
			if raw == "" {
				continue
			}
			quantity, err := strconv.Atoi(raw)
			if err != nil || quantity < 1 {
				delete(cart, slug)
				continue
			}
			if stock, _, ok := productStock(a, r, slug); ok && quantity > stock {
				quantity = stock
			}
			cart[slug] = quantity
		}
		saveCart(r, cart)
		view.Success(r, "Cart updated.")
		view.Redirect(w, r, "/cart")
	}
}

func cartRemove(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		cart := cartOf(r)
		delete(cart, r.PostForm.Get("product"))
		saveCart(r, cart)
		view.Success(r, "Removed from your cart.")
		view.Redirect(w, r, "/cart")
	}
}

func productStock(a *app.App, r *http.Request, slug string) (int, string, bool) {
	records, err := a.Store()
	if err != nil {
		return 0, "", false
	}
	schema, err := a.Describe(Product{})
	if err != nil {
		return 0, "", false
	}
	found, err := records.First(r.Context(), schema, model.Query{
		Filters: []model.Filter{
			{Column: "slug", Op: model.Eq, Value: slug},
			{Column: "is_active", Op: model.Eq, Value: true},
		},
	})
	if err != nil {
		return 0, "", false
	}
	return int(recordInt(found, "stock")), found.String("name"), true
}
