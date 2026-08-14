package main

import (
	"log"
	"net/http"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
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

	application.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		application.Render(w, r, "pages/home.html", view.Data{"Title": "Home", "HideNav": true})
	})

	application.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		application.Render(w, r, "pages/about.html", view.Data{
			"Title":        "About",
			"DatabaseName": application.Settings.Database().Name,
		})
	})

	application.Get("/notes", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		notes, _ := sess.Get("notes").([]string)
		application.Render(w, r, "pages/notes.html", view.Data{
			"Title": "Session notes",
			"Notes": notes,
		})
	}, application.CSRF)

	application.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		note := r.PostForm.Get("note")
		if note == "" {
			view.Flash(r, "error", "Write something first.")
			view.Redirect(w, r, "/notes")
			return
		}
		sess := session.FromRequest(r)
		notes, _ := sess.Get("notes").([]string)
		sess.Set("notes", append(notes, note))
		view.Flash(r, "success", "Note saved to your session.")
		view.Redirect(w, r, "/notes")
	}, application.CSRF)

	private := application.Group("/me", application.Auth.RequireLogin(application.Settings.Auth.LoginURL))
	private.Get("/", func(w http.ResponseWriter, r *http.Request) {
		application.Render(w, r, "pages/profile.html", view.Data{"Title": "Your profile"})
	})

	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
