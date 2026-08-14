package tests

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/settings"
)

func limited(policy settings.RateLimit) http.Handler {
	return middleware.RateLimit(policy)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
}

func from(t *testing.T, handler http.Handler, ip string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitAllowsUpToTheLimit(t *testing.T) {
	handler := limited(settings.RateLimit{Requests: 3, Window: time.Minute})

	for i := 1; i <= 3; i++ {
		rec := from(t, handler, "10.0.0.1")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i, rec.Code)
		}
	}
	rec := from(t, handler, "10.0.0.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the fourth request = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Errorf("Remaining = %q", rec.Header().Get("RateLimit-Remaining"))
	}
}

func TestRateLimitIsPerClient(t *testing.T) {
	handler := limited(settings.RateLimit{Requests: 2, Window: time.Minute})

	for i := 0; i < 2; i++ {
		from(t, handler, "10.0.0.1")
	}
	if rec := from(t, handler, "10.0.0.1"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the first client should be limited, got %d", rec.Code)
	}
	if rec := from(t, handler, "10.0.0.2"); rec.Code != http.StatusOK {
		t.Errorf("a different client must not be affected, got %d", rec.Code)
	}
}

func TestRateLimitRefillsOverTime(t *testing.T) {
	handler := limited(settings.RateLimit{Requests: 20, Window: 100 * time.Millisecond})

	for i := 0; i < 20; i++ {
		from(t, handler, "10.0.0.3")
	}
	if rec := from(t, handler, "10.0.0.3"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected to be limited, got %d", rec.Code)
	}

	time.Sleep(60 * time.Millisecond)
	if rec := from(t, handler, "10.0.0.3"); rec.Code != http.StatusOK {
		t.Errorf("tokens should refill over time, got %d", rec.Code)
	}
}

func TestRateLimitBurst(t *testing.T) {
	handler := limited(settings.RateLimit{Requests: 2, Window: time.Second, Burst: 5})

	for i := 1; i <= 5; i++ {
		if rec := from(t, handler, "10.0.0.4"); rec.Code != http.StatusOK {
			t.Fatalf("burst request %d = %d, want 200", i, rec.Code)
		}
	}
	if rec := from(t, handler, "10.0.0.4"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("beyond the burst should be limited, got %d", rec.Code)
	}
}

func TestRateLimitHeadersCountDown(t *testing.T) {
	handler := limited(settings.RateLimit{Requests: 3, Window: time.Minute})

	previous := 99
	for i := 0; i < 3; i++ {
		rec := from(t, handler, "10.0.0.5")
		if rec.Header().Get("RateLimit-Limit") != "3" {
			t.Errorf("Limit = %q", rec.Header().Get("RateLimit-Limit"))
		}
		remaining, err := strconv.Atoi(rec.Header().Get("RateLimit-Remaining"))
		if err != nil {
			t.Fatal(err)
		}
		if remaining >= previous {
			t.Errorf("remaining did not decrease: %d then %d", previous, remaining)
		}
		previous = remaining
	}
}

func TestRateLimitTrustsProxyOnlyWhenAsked(t *testing.T) {
	untrusted := limited(settings.RateLimit{Requests: 1, Window: time.Minute})
	for _, forwarded := range []string{"1.1.1.1", "2.2.2.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.6:1234"
		req.Header.Set("X-Forwarded-For", forwarded)
		rec := httptest.NewRecorder()
		untrusted.ServeHTTP(rec, req)
		if forwarded == "2.2.2.2" && rec.Code != http.StatusTooManyRequests {
			t.Error("spoofed forwarding headers must not reset the limit")
		}
	}

	trusted := limited(settings.RateLimit{Requests: 1, Window: time.Minute, TrustProxy: true})
	for _, forwarded := range []string{"1.1.1.1", "2.2.2.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.7:1234"
		req.Header.Set("X-Forwarded-For", forwarded+", 10.0.0.7")
		rec := httptest.NewRecorder()
		trusted.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("distinct forwarded clients should each get their own budget, got %d", rec.Code)
		}
	}
}

func TestRateLimitCustomKey(t *testing.T) {
	handler := middleware.RateLimitBy(
		settings.RateLimit{Requests: 1, Window: time.Minute},
		func(r *http.Request) string { return r.Header.Get("X-Tenant") },
	)(http.HandlerFunc(noop))

	first := httptest.NewRequest(http.MethodGet, "/", nil)
	first.Header.Set("X-Tenant", "acme")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, first)
	if rec.Code != http.StatusOK {
		t.Fatal("the first request should pass")
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, first)
	if rec.Code != http.StatusTooManyRequests {
		t.Error("the same tenant should be limited")
	}

	other := httptest.NewRequest(http.MethodGet, "/", nil)
	other.Header.Set("X-Tenant", "globex")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, other)
	if rec.Code != http.StatusOK {
		t.Error("a different tenant should have its own budget")
	}
}

func TestRateLimitIsConcurrencySafe(t *testing.T) {
	handler := limited(settings.RateLimit{Requests: 50, Window: time.Minute})

	var wait sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 100; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "10.0.0.9:1234"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			mu.Lock()
			if rec.Code == http.StatusOK {
				allowed++
			}
			mu.Unlock()
		}()
	}
	wait.Wait()

	if allowed > 51 {
		t.Errorf("%d requests were allowed, want no more than the 50 budget", allowed)
	}
	if allowed == 0 {
		t.Error("nothing was allowed at all")
	}
}

func TestRateLimitIsOffByDefault(t *testing.T) {
	a := newTestApp(t)
	a.Get("/x", noop)
	for i := 0; i < 30; i++ {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d was limited without a policy", i)
		}
	}
}

func TestRateLimitFromSettings(t *testing.T) {
	a := newTestApp(t, func(s *settings.Settings) {
		s.Security.RateLimit = settings.RateLimit{Requests: 2, Window: time.Minute}
	})
	a.Get("/x", noop)

	codes := []int{}
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "10.1.1.1:9999"
		a.Handler().ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Errorf("codes = %v, want [200 200 429]", codes)
	}
}

func TestRateLimitValidation(t *testing.T) {
	cases := map[string]settings.RateLimit{
		"without a Window":            {Requests: 10},
		"without Requests":            {Window: time.Minute},
		"Requests cannot be negative": {Requests: -1, Window: time.Minute},
		"Burst cannot be negative":    {Requests: 1, Window: time.Minute, Burst: -5},
		"smaller than Requests":       {Requests: 10, Window: time.Minute, Burst: 2},
	}
	for want, policy := range cases {
		_, err := settings.New(prodSettings(func(s *settings.Settings) { s.Security.RateLimit = policy })...)
		if err == nil {
			t.Errorf("expected an error mentioning %q", want)
			continue
		}
		mustContain(t, problemsOf(t, err), want)
	}
}
