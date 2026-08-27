package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/app"
)

func appWithACustomNotFound(t *testing.T) *app.App {
	t.Helper()

	a := newTestApp(t)
	a.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("about page"))
	})
	a.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("item " + r.PathValue("id")))
	})
	a.Get("/gone", func(w http.ResponseWriter, r *http.Request) {
		a.RenderStatus(w, r, http.StatusNotFound, "pages/hello.html", app.Data{"Name": "deliberate"})
	})
	a.SetNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("custom not found for " + r.URL.Path))
	}))
	return a
}

func TestACustomNotFoundHandlesUnroutedPaths(t *testing.T) {
	client := newClient(t, appWithACustomNotFound(t).Handler())

	response := client.get("/no-such-page")
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
	if !strings.Contains(response.Body.String(), "custom not found for /no-such-page") {
		t.Errorf("the custom handler did not run: %q", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "404 page not found") {
		t.Error("the standard library's plain text 404 reached the client")
	}
}

func TestACustomNotFoundLeavesMatchedRoutesAlone(t *testing.T) {
	client := newClient(t, appWithACustomNotFound(t).Handler())

	for path, expected := range map[string]string{
		"/about":    "about page",
		"/items/42": "item 42",
	} {
		response := client.get(path)
		if response.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, response.Code)
		}
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("%s: body = %q, want %q", path, response.Body.String(), expected)
		}
	}
}

func TestACustomNotFoundDoesNotSwallowAMethodMismatch(t *testing.T) {
	client := newClient(t, appWithACustomNotFound(t).Handler())

	response := client.do(http.MethodPost, "/about", nil)
	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405; a method mismatch is not a missing page", response.Code)
	}
	if strings.Contains(response.Body.String(), "custom not found") {
		t.Error("the not-found handler ran for a method mismatch")
	}
}

func TestACustomNotFoundDoesNotSwallowAHandlersOwn404(t *testing.T) {
	client := newClient(t, appWithACustomNotFound(t).Handler())

	response := client.get("/gone")
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
	if strings.Contains(response.Body.String(), "custom not found") {
		t.Error("a handler that renders its own 404 was replaced by the fallback")
	}
	if !strings.Contains(response.Body.String(), "deliberate") {
		t.Errorf("the handler's own page did not render: %q", response.Body.String())
	}
}

func TestWithoutACustomNotFoundNothingChanges(t *testing.T) {
	a := newTestApp(t)
	a.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("about page"))
	})

	client := newClient(t, a.Handler())

	if response := client.get("/about"); response.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", response.Code)
	}
	if response := client.get("/missing"); response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
	if response := client.do(http.MethodPost, "/about", nil); response.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", response.Code)
	}
}

func TestACustomNotFoundAppliesInsideAGroup(t *testing.T) {
	a := newTestApp(t)
	group := a.Group("/api")
	group.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	a.SetNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("custom not found"))
	}))

	client := newClient(t, a.Handler())

	if response := client.get("/api/ping"); response.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", response.Code)
	}

	response := client.get("/api/nope")
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
	if !strings.Contains(response.Body.String(), "custom not found") {
		t.Errorf("an unmatched path under a group should reach the fallback: %q", response.Body.String())
	}
}
