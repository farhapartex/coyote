package main

import (
	"log"
	"net/http"

	"github.com/farhapartex/coyote/contrib/accounts"
	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/session"

	_ "shop/migrations"
)

func init() {
	session.RegisterValue(map[string]int{})
}

func main() {
	a := app.New()

	a.RegisterModel(
		model.Of(Category{}),
		model.Of(Product{}),
		model.Of(Address{}),
		model.Of(Coupon{}),
		model.Of(Order{}),
		model.Of(OrderLine{}),
		model.Of(ShipmentEvent{}),
		model.Of(StockMovement{}),
	)

	registerDeliveryJob(a)

	portal := admin.Mount(a)
	portal.MustManage(
		categoryResource{},
		productResource{},
		orderResource{a: a},
		orderLineResource{},
		shipmentEventResource{},
		couponResource{},
		stockMovementResource{},
		addressResource{},
	)

	accounts.Mount(a, accounts.Options{
		Prefix:            "/accounts",
		AllowRegistration: true,
		AfterLogin:        "/",
		AfterLogout:       "/",
	})

	a.Get("/{$}", landingPage(a)).Named("home")
	a.Get("/products", productList(a)).Named("products")
	a.Get("/products/{slug}", productPage(a)).Named("product")
	a.Get("/categories/{slug}", categoryPage(a)).Named("category")

	a.Get("/cart", cartPage(a)).Named("cart")
	a.Post("/cart/add", cartAdd(a)).Named("cart.add")
	a.Post("/cart/update", cartUpdate(a)).Named("cart.update")
	a.Post("/cart/remove", cartRemove(a)).Named("cart.remove")

	a.Get("/checkout", checkoutPage(a)).Named("checkout")
	a.Post("/checkout", checkoutSubmit(a))
	a.Post("/checkout/coupon", couponApply(a)).Named("checkout.coupon")

	guarded := a.Group("", a.Auth.RequireLogin("/accounts/login"))
	guarded.Get("/orders", orderList(a)).Named("orders")
	guarded.Get("/orders/{reference}", orderPage(a)).Named("order")

	a.SetNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notFound(a, w, r)
	}))

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
