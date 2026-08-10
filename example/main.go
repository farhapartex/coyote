package main

import (
	"log"
	"net/http"
	"time"

	"github.com/farhapartex/coyote"
	"github.com/farhapartex/coyote/admin"
	"github.com/farhapartex/coyote/session"
)

func main() {
	app := coyote.New()

	if _, err := app.Auth.CreateUser("admin", "admin@example.com", "coyote123", true, true); err != nil {
		log.Fatalf("seeding admin user: %v", err)
	}
	if _, err := app.Auth.CreateUser("editor", "editor@example.com", "coyote123", true, false); err != nil {
		log.Fatalf("seeding editor user: %v", err)
	}

	portal := admin.Mount(app)
	portal.Register(admin.Section{
		Name:        "Server info",
		Slug:        "server-info",
		Description: "A custom admin page contributed by the application.",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app.Render(w, r, "pages/server_info.html", coyote.Data{
				"Title":  "Server info",
				"Uptime": time.Since(app.Started).Round(time.Second).String(),
				"Routes": app.Routes(),
			})
		}),
	})

	app.Get("/", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		visits := sess.GetInt("visits") + 1
		sess.Set("visits", visits)
		app.Render(w, r, "pages/home.html", coyote.Data{
			"Title":  "Home",
			"Visits": visits,
		})
	})

	app.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		app.Render(w, r, "pages/about.html", coyote.Data{"Title": "About"})
	})

	app.Get("/notes", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		notes, _ := sess.Get("notes").([]string)
		app.Render(w, r, "pages/notes.html", coyote.Data{
			"Title": "Session notes",
			"Notes": notes,
		})
	}, app.CSRF)

	app.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}
		note := r.PostForm.Get("note")
		if note == "" {
			coyote.Flash(r, "error", "Write something first.")
			coyote.Redirect(w, r, "/notes")
			return
		}
		sess := session.FromRequest(r)
		notes, _ := sess.Get("notes").([]string)
		sess.Set("notes", append(notes, note))
		coyote.Flash(r, "success", "Note saved to your session.")
		coyote.Redirect(w, r, "/notes")
	}, app.CSRF)

	private := app.Group("/me", app.Auth.RequireLogin(app.Settings.Auth.LoginURL))
	private.Get("/", func(w http.ResponseWriter, r *http.Request) {
		app.Render(w, r, "pages/profile.html", coyote.Data{"Title": "Your profile"})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
