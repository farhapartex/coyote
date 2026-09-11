package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/view"
	"github.com/farhapartex/coyote/lib/id"
)

const couponKey = "shop_coupon"

func couponInSession(a *app.App, r *http.Request) (string, int) {
	sess := session.FromRequest(r)
	if sess == nil {
		return "", 0
	}
	code := sess.GetString(couponKey)
	if code == "" {
		return "", 0
	}
	percent, ok := couponPercent(a, r, code)
	if !ok {
		return "", 0
	}
	return code, percent
}

func couponPercent(a *app.App, r *http.Request, code string) (int, bool) {
	records, err := a.Store()
	if err != nil {
		return 0, false
	}
	schema, err := a.Describe(Coupon{})
	if err != nil {
		return 0, false
	}
	found, err := records.First(r.Context(), schema, model.Query{
		Filters: []model.Filter{
			{Column: "code", Op: model.Eq, Value: strings.ToUpper(strings.TrimSpace(code))},
			{Column: "is_active", Op: model.Eq, Value: true},
		},
	})
	if err != nil {
		return 0, false
	}
	return int(recordInt(found, "percent_off")), true
}

func couponApply(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		code := strings.ToUpper(strings.TrimSpace(r.PostForm.Get("code")))
		sess := session.FromRequest(r)

		if code == "" {
			if sess != nil {
				sess.Delete(couponKey)
			}
			view.Success(r, "Coupon removed.")
			view.Redirect(w, r, "/checkout")
			return
		}
		percent, ok := couponPercent(a, r, code)
		if !ok {
			view.Error(r, "That coupon is not valid.")
			view.Redirect(w, r, "/checkout")
			return
		}
		if sess != nil {
			sess.Set(couponKey, code)
		}
		view.Success(r, fmt.Sprintf("%s applied, %d%% off.", code, percent))
		view.Redirect(w, r, "/checkout")
	}
}

func checkoutPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		basket, err := loadCart(a, r)
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		if len(basket.Lines) == 0 {
			view.Warning(r, "Your cart is empty.")
			view.Redirect(w, r, "/products")
			return
		}
		user := a.Auth.CurrentUser(r)
		render(a, w, r, "pages/checkout.html", app.Data{
			"Title":    "Checkout",
			"Nav":      "checkout",
			"Cart":     basket,
			"Problems": map[string]string{},
			"Form":     checkoutFormOf(r, user),
		})
	}
}

type checkoutForm struct {
	Email    string
	Line1    string
	Line2    string
	City     string
	Postcode string
}

func checkoutFormOf(r *http.Request, user *auth.User) checkoutForm {
	out := checkoutForm{}
	if user != nil {
		out.Email = user.Email
	}
	if r.Method == http.MethodPost {
		out.Email = strings.TrimSpace(r.PostForm.Get("email"))
		out.Line1 = strings.TrimSpace(r.PostForm.Get("line1"))
		out.Line2 = strings.TrimSpace(r.PostForm.Get("line2"))
		out.City = strings.TrimSpace(r.PostForm.Get("city"))
		out.Postcode = strings.TrimSpace(r.PostForm.Get("postcode"))
	}
	return out
}

func checkoutSubmit(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		user := a.Auth.CurrentUser(r)
		if user == nil {
			view.Warning(r, "Please sign in to place your order.")
			view.Redirect(w, r, "/accounts/login?next=%2Fcheckout")
			return
		}

		basket, err := loadCart(a, r)
		if err != nil {
			serverError(a, w, r, err)
			return
		}
		if len(basket.Lines) == 0 {
			view.Warning(r, "Your cart is empty.")
			view.Redirect(w, r, "/products")
			return
		}

		form := checkoutFormOf(r, user)
		problems := map[string]string{}
		if form.Email == "" || !strings.Contains(form.Email, "@") {
			problems["email"] = "We need an email address to send the receipt to."
		}
		if form.Line1 == "" {
			problems["line1"] = "A street address is required."
		}
		if form.City == "" {
			problems["city"] = "A town or city is required."
		}
		if form.Postcode == "" {
			problems["postcode"] = "A postcode is required."
		}
		if len(problems) > 0 {
			renderStatus(a, w, r, http.StatusUnprocessableEntity, "pages/checkout.html", app.Data{
				"Title":    "Checkout",
				"Nav":      "checkout",
				"Cart":     basket,
				"Problems": problems,
				"Form":     form,
			})
			return
		}

		reference, err := placeOrder(a, r, user, basket, form)
		if err != nil {
			a.Logger.Error("checkout failed", "error", err)
			view.Error(r, "We could not place your order: "+err.Error())
			view.Redirect(w, r, "/checkout")
			return
		}

		saveCart(r, map[string]int{})
		if sess := session.FromRequest(r); sess != nil {
			sess.Delete(couponKey)
		}
		view.Success(r, "Order "+reference+" placed. Thank you.")
		view.Redirect(w, r, "/orders/"+reference)
	}
}

func placeOrder(a *app.App, r *http.Request, user *auth.User, basket cartView, form checkoutForm) (string, error) {
	reference := orderReference()
	shipTo := strings.Join(nonEmpty(form.Line1, form.Line2, form.City, form.Postcode), ", ")
	now := time.Now().UTC()

	err := a.Transaction(r.Context(), func(tx *app.Tx) error {
		records := tx.Store()

		orderSchema, err := a.Describe(Order{})
		if err != nil {
			return err
		}
		lineSchema, err := a.Describe(OrderLine{})
		if err != nil {
			return err
		}
		eventSchema, err := a.Describe(ShipmentEvent{})
		if err != nil {
			return err
		}
		productSchema, err := a.Describe(Product{})
		if err != nil {
			return err
		}
		movementSchema, err := a.Describe(StockMovement{})
		if err != nil {
			return err
		}

		discount := basket.GoodsCents * int64(basket.PercentOff) / 100
		orderID := id.MustNew()
		if _, err := records.Insert(r.Context(), orderSchema, model.Record{
			"id":             orderID,
			"reference":      reference,
			"user_id":        user.ID,
			"state":          OrderPlaced,
			"email":          form.Email,
			"ship_to":        shipTo,
			"coupon_code":    basket.Coupon,
			"goods_cents":    basket.GoodsCents,
			"discount_cents": discount,
			"shipping_cents": int64(shippingCents),
			"total_cents":    basket.TotalCents,
			"placed_at":      now,
		}); err != nil {
			return err
		}

		for _, line := range basket.Lines {
			found, err := records.First(r.Context(), productSchema, model.Query{
				Filters: []model.Filter{{Column: "slug", Op: model.Eq, Value: line.Slug}},
			})
			if err != nil {
				return err
			}
			productID := found.String("id")
			stock := int(recordInt(found, "stock"))
			if stock < line.Quantity {
				return fmt.Errorf("%s has only %d left", line.Name, stock)
			}

			if _, err := records.Insert(r.Context(), lineSchema, model.Record{
				"id":         id.MustNew(),
				"order_id":   orderID,
				"product_id": productID,
				"title":      line.Name,
				"quantity":   line.Quantity,
				"unit_cents": found.Get("price_cents"),
				"line_cents": line.Cents,
			}); err != nil {
				return err
			}
			if err := records.Update(r.Context(), productSchema, productID, model.Record{
				"stock": stock - line.Quantity,
			}); err != nil {
				return err
			}
			if _, err := records.Insert(r.Context(), movementSchema, model.Record{
				"id":         id.MustNew(),
				"product_id": productID,
				"delta":      -line.Quantity,
				"reason":     "order " + reference,
			}); err != nil {
				return err
			}
		}

		if _, err := records.Insert(r.Context(), eventSchema, model.Record{
			"id":          id.MustNew(),
			"order_id":    orderID,
			"state":       OrderPlaced,
			"note":        "We have your order and your payment.",
			"happened_at": now,
		}); err != nil {
			return err
		}

		_, err = jobs.Enqueue(r.Context(), tx.Queue(), JobAdvanceOrder, advanceArgs{Reference: reference},
			jobs.Options{Delay: 2 * time.Second})
		return err
	})
	if err != nil {
		return "", err
	}
	return reference, nil
}

func orderReference() string {
	raw, err := id.Short()
	if err != nil {
		return "TS" + time.Now().UTC().Format("150405")
	}
	return "TS-" + strings.ToUpper(raw[:8])
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}
