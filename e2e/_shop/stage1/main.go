package main

import (
	"log"
	"net/http"

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
	)

	portal := admin.Mount(a)
	portal.MustManage(categoryResource{}, productResource{})

	a.Get("/{$}", landingPage(a)).Named("home")
	a.Get("/products", productList(a)).Named("products")
	a.Get("/products/{slug}", productPage(a)).Named("product")
	a.Get("/categories/{slug}", categoryPage(a)).Named("category")

	a.SetNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notFound(a, w, r)
	}))

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
