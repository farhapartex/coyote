package main

import (
	"log"
	"net/http"

	"github.com/farhapartex/coyote/contrib/accounts"
	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/view"

	_ "github.com/farhapartex/coyote/example/migrations"
)

func main() {
	application := app.New()

	application.RegisterModel(model.Of(Product{}), model.Of(Checkout{}))

	portal := admin.Mount(application)
	portal.MustManage(productResource{}, checkoutResource{})

	accounts.Mount(application, accounts.Options{
		AllowRegistration: true,
		AfterLogin:        "/me/",
	})

	application.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		application.Render(w, r, "pages/home.html", view.Data{"Title": i18n.T(r.Context(), "Home"), "HideNav": true})
	}).Named("home")

	application.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		application.Render(w, r, "pages/about.html", view.Data{
			"Title":        i18n.T(r.Context(), "About"),
			"DatabaseName": application.Settings.Database().Name,
		})
	}).Named("about")

	application.Get("/notes", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		notes, _ := sess.Get("notes").([]string)
		application.Render(w, r, "pages/notes.html", view.Data{
			"Title": i18n.T(r.Context(), "Session notes"),
			"Notes": notes,
		})
	}, application.CSRF).Named("notes")

	application.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		note := r.PostForm.Get("note")
		if note == "" {
			view.Flash(r, "error", i18n.T(r.Context(), "Write something first."))
			view.Redirect(w, r, "/notes")
			return
		}
		sess := session.FromRequest(r)
		notes, _ := sess.Get("notes").([]string)
		sess.Set("notes", append(notes, note))
		view.Flash(r, "success", i18n.T(r.Context(), "Note saved to your session."))
		view.Redirect(w, r, "/notes")
	}, application.CSRF)

	private := application.Group("/me", application.Auth.RequireLogin(application.Settings.Auth.LoginURL))
	private.Get("/", func(w http.ResponseWriter, r *http.Request) {
		application.Render(w, r, "pages/profile.html", view.Data{"Title": i18n.T(r.Context(), "Your profile")})
	})

	reports := application.Group("/reports", application.Auth.RequirePermission("products.read"))
	reports.Get("/{$}", reportHandler(application)).Named("reports")

	application.Get("/locale", application.Locales().SwitchHandler("/"))
	application.Post("/locale", application.Locales().SwitchHandler("/"), application.CSRF)

	application.Get("/cached", cachedPage(application), application.CSRF).Named("cached")
	application.Post("/cached", cachedSave(application), application.CSRF)

	sender, err := mailSender(application)
	if err != nil {
		log.Fatal(err)
	}
	outbox := counting(sender)

	application.Get("/contact", contactPage(application), application.CSRF).Named("contact")
	application.Post("/contact", contactSend(application, outbox), application.CSRF)

	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
