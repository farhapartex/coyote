package main

import (
	"log"
	"net/http"
	"time"

	"github.com/farhapartex/coyote/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/session"
	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/core/view"
)

func main() {
	application := app.New()

	if _, err := application.Auth.CreateUser(auth.NewUser{
		Username:     "admin",
		Email:        "admin@example.com",
		FirstName:    "Ada",
		LastName:     "Lovelace",
		Password:     "coyote123",
		IsSuperadmin: true,
	}); err != nil {
		log.Fatalf("seeding admin user: %v", err)
	}
	if _, err := application.Auth.CreateUser(auth.NewUser{
		Username:  "editor",
		Email:     "editor@example.com",
		FirstName: "Grace",
		LastName:  "Hopper",
		Password:  "coyote123",
	}); err != nil {
		log.Fatalf("seeding editor user: %v", err)
	}

	portal := admin.Mount(application)
	portal.Register(admin.Section{
		Name:        "Server info",
		Slug:        "server-info",
		Description: "A custom admin page contributed by the application.",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			application.Render(w, r, "pages/server_info.html", view.Data{
				"Title":     "Server info",
				"Uptime":    time.Since(application.Started).Round(time.Second).String(),
				"Routes":    application.Routes(),
				"BaseDir":   application.Settings.BaseDir,
				"Databases": redactedDatabases(application.Settings),
			})
		}),
	})

	application.Get("/", func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromRequest(r)
		visits := sess.GetInt("visits") + 1
		sess.Set("visits", visits)
		application.Render(w, r, "pages/home.html", view.Data{
			"Title":  "Home",
			"Visits": visits,
		})
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

func redactedDatabases(s settings.Settings) []settings.Database {
	out := make([]settings.Database, 0, len(s.Databases))
	for _, db := range s.Databases {
		out = append(out, db.Redacted())
	}
	return out
}
