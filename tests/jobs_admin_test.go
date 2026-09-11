package tests

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/settings"
	"gorm.io/gorm"
)

func newJobAdmin(t *testing.T, fns ...func(*settings.Settings)) (*app.App, *client, *gorm.DB) {
	t.Helper()
	a := newJobApp(t, append([]func(*settings.Settings){
		func(s *settings.Settings) { s.Admin.SiteName = "Test admin" },
	}, fns...)...)
	if _, err := a.Auth.CreateSuperadmin(t.Context(), "root", "root@example.com", "supersecret"); err != nil {
		t.Fatalf("creating a superadmin: %v", err)
	}
	if _, err := a.Auth.CreateUser(t.Context(), auth.NewUser{
		Username: "clerk", Password: "supersecret", IsStaff: true,
	}); err != nil {
		t.Fatalf("creating a staff user: %v", err)
	}
	admin.Mount(a)

	handle, err := a.DB()
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	return a, newClient(t, a.Handler()), handle
}

func buryAJob(t *testing.T, a *app.App, kind, cause string) string {
	t.Helper()
	id, err := jobs.Enqueue(t.Context(), a.Queue(), kind, nil, jobs.Options{MaxAttempts: 1, Priority: 100})
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if _, err := a.Queue().Claim(t.Context(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if err := a.Queue().Fail(t.Context(), id, cause, nil); err != nil {
		t.Fatalf("failing: %v", err)
	}
	return id
}

func TestTheJobsPageListsJobsWithTheirStateAndError(t *testing.T) {
	a, browser, _ := newJobAdmin(t)
	signInAsRoot(t, browser)
	buryAJob(t, a, "email.welcome", "the mailbox is full")

	body := browser.get("/admin/jobs").Body.String()
	for _, want := range []string{"email.welcome", "the mailbox is full", "dead", "Background jobs"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the jobs page does not mention %q", want)
		}
	}
}

func TestTheJobsPageIsAbsentWhileJobsAreOff(t *testing.T) {
	_, browser := setupAdmin(t)
	signInAsRoot(t, browser)

	response := browser.get("/admin/jobs")
	if response.Code != http.StatusNotFound {
		t.Fatalf("the jobs page answered %d while jobs are off, want 404", response.Code)
	}
	if strings.Contains(browser.get("/admin/").Body.String(), "/admin/jobs") {
		t.Fatal("the navigation links to the jobs page while jobs are off")
	}
}

func TestTheNavigationLinksToJobsWhenTheyAreOn(t *testing.T) {
	_, browser, _ := newJobAdmin(t)
	signInAsRoot(t, browser)

	if !strings.Contains(browser.get("/admin/").Body.String(), "/admin/jobs") {
		t.Fatal("the navigation does not link to the jobs page")
	}
}

func TestOnlyASuperadminReachesTheJobsPage(t *testing.T) {
	_, browser, _ := newJobAdmin(t)
	if res := browser.login("/admin/login", "clerk", "supersecret"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("the staff sign-in answered %d", res.StatusCode)
	}

	if response := browser.get("/admin/jobs"); response.Code == http.StatusOK {
		t.Fatal("a staff user without superadmin reached the jobs page")
	}
}

func TestAnonymousVisitorsAreSentToTheLogin(t *testing.T) {
	_, browser, _ := newJobAdmin(t)

	response := browser.get("/admin/jobs")
	if response.Code != http.StatusSeeOther {
		t.Fatalf("an anonymous request answered %d, want a redirect to the login", response.Code)
	}
}

func TestRetryingADeadJobPutsItBackInTheQueue(t *testing.T) {
	a, browser, handle := newJobAdmin(t)
	signInAsRoot(t, browser)
	id := buryAJob(t, a, "email.welcome", "the mailbox is full")

	response := browser.do(http.MethodPost, "/admin/jobs/"+id+"/retry", url.Values{
		"csrf_token": {browser.token("/admin/jobs")},
	})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the retry answered %d, want a redirect", response.Code)
	}

	stored := storedJob(t, handle, id)
	if stored.State != jobs.Queued {
		t.Fatalf("the retried job is %q, want queued", stored.State)
	}
	if stored.Attempts != 0 || stored.LastError != "" || stored.FinishedAt != nil {
		t.Fatalf("the retried job kept its history: %+v", stored)
	}
}

func TestRetryingWithoutACSRFTokenIsRefused(t *testing.T) {
	a, browser, handle := newJobAdmin(t)
	signInAsRoot(t, browser)
	id := buryAJob(t, a, "email.welcome", "no")

	if response := browser.do(http.MethodPost, "/admin/jobs/"+id+"/retry", url.Values{}); response.Code != http.StatusForbidden {
		t.Fatalf("a retry with no CSRF token answered %d, want 403", response.Code)
	}
	if storedJob(t, handle, id).State != jobs.Dead {
		t.Fatal("the job was retried despite the missing token")
	}
}

func TestARunningJobCannotBeRetried(t *testing.T) {
	a, browser, handle := newJobAdmin(t)
	signInAsRoot(t, browser)

	id, err := jobs.Enqueue(t.Context(), a.Queue(), "long.one", nil)
	if err != nil {
		t.Fatalf("enqueueing: %v", err)
	}
	if _, err := a.Queue().Claim(t.Context(), "worker-1", nil, 1); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	browser.do(http.MethodPost, "/admin/jobs/"+id+"/retry", url.Values{
		"csrf_token": {browser.token("/admin/jobs")},
	})
	if stored := storedJob(t, handle, id); stored.State != jobs.Running {
		t.Fatalf("a running job became %q; retrying it would duplicate the work", stored.State)
	}
}

func TestBulkRetryQueuesEverySelectedJob(t *testing.T) {
	a, browser, handle := newJobAdmin(t)
	signInAsRoot(t, browser)
	first := buryAJob(t, a, "one", "no")
	second := buryAJob(t, a, "two", "no")

	response := browser.do(http.MethodPost, "/admin/jobs/bulk", url.Values{
		"csrf_token": {browser.token("/admin/jobs")},
		"action":     {"retry"},
		"ids":        {first, second},
	})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the bulk retry answered %d", response.Code)
	}
	for _, id := range []string{first, second} {
		if stored := storedJob(t, handle, id); stored.State != jobs.Queued {
			t.Fatalf("job %s is %q, want queued", id, stored.State)
		}
	}
}

func TestBulkDeleteRemovesEverySelectedJob(t *testing.T) {
	a, browser, handle := newJobAdmin(t)
	signInAsRoot(t, browser)
	first := buryAJob(t, a, "one", "no")
	second := buryAJob(t, a, "two", "no")

	response := browser.do(http.MethodPost, "/admin/jobs/bulk", url.Values{
		"csrf_token": {browser.token("/admin/jobs")},
		"action":     {"delete"},
		"ids":        {first, second},
	})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the bulk delete answered %d", response.Code)
	}

	var remaining int64
	if err := handle.Model(&jobs.Record{}).Count(&remaining).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("%d jobs remain after deleting both", remaining)
	}
}

func TestAnUnknownBulkActionChangesNothing(t *testing.T) {
	a, browser, handle := newJobAdmin(t)
	signInAsRoot(t, browser)
	id := buryAJob(t, a, "one", "no")

	browser.do(http.MethodPost, "/admin/jobs/bulk", url.Values{
		"csrf_token": {browser.token("/admin/jobs")},
		"action":     {"purge-everything"},
		"ids":        {id},
	})
	if stored := storedJob(t, handle, id); stored.State != jobs.Dead {
		t.Fatalf("an unknown action changed the job to %q", stored.State)
	}
}

func TestTheStateFilterNarrowsTheList(t *testing.T) {
	a, browser, _ := newJobAdmin(t)
	signInAsRoot(t, browser)
	buryAJob(t, a, "dead.one", "no")
	if _, err := jobs.Enqueue(t.Context(), a.Queue(), "queued.one", nil); err != nil {
		t.Fatalf("enqueueing: %v", err)
	}

	dead := browser.get("/admin/jobs?state=dead").Body.String()
	if !strings.Contains(dead, "dead.one") || strings.Contains(dead, "queued.one") {
		t.Fatal("the dead filter did not narrow the list")
	}

	queued := browser.get("/admin/jobs?state=queued").Body.String()
	if !strings.Contains(queued, "queued.one") || strings.Contains(queued, "dead.one") {
		t.Fatal("the queued filter did not narrow the list")
	}
}

func TestAnInjectedStateFilterIsIgnored(t *testing.T) {
	a, browser, _ := newJobAdmin(t)
	signInAsRoot(t, browser)
	buryAJob(t, a, "dead.one", "no")

	for _, probe := range []string{"'; drop table jobs; --", "queued' or '1'='1", "../../etc", ""} {
		response := browser.get("/admin/jobs?state=" + url.QueryEscape(probe))
		if response.Code != http.StatusOK {
			t.Fatalf("state=%q answered %d", probe, response.Code)
		}
		if !strings.Contains(response.Body.String(), "dead.one") {
			t.Fatalf("state=%q hid every row instead of falling back to all of them", probe)
		}
	}
}

func TestJobsCannotBeRegisteredAsAResourceUnderTheReservedSlug(t *testing.T) {
	a := newJobApp(t)
	portal := admin.Mount(a)

	if err := portal.Manage(jobsResource{}); err == nil {
		t.Fatal("a resource was allowed to take over the /admin/jobs path")
	}
}

type jobsResource struct{}

func (jobsResource) Entity() any { return jobs.Record{} }

func TestTheJobsPageShowsTheQueueCounts(t *testing.T) {
	a, browser, _ := newJobAdmin(t)
	signInAsRoot(t, browser)

	for range 3 {
		if _, err := jobs.Enqueue(t.Context(), a.Queue(), "waiting", nil); err != nil {
			t.Fatalf("enqueueing: %v", err)
		}
	}
	buryAJob(t, a, "gone", "no")

	body := browser.get("/admin/jobs").Body.String()
	if !strings.Contains(body, "Queued") || !strings.Contains(body, "Dead") {
		t.Fatal("the jobs page shows no state counts")
	}
}

func TestAnEmptyQueueRendersWithoutAnError(t *testing.T) {
	_, browser, _ := newJobAdmin(t)
	signInAsRoot(t, browser)

	response := browser.get("/admin/jobs")
	if response.Code != http.StatusOK {
		t.Fatalf("an empty queue answered %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "No jobs to show") {
		t.Fatal("an empty queue does not say so")
	}
}

func signInAsRoot(t *testing.T, c *client) {
	t.Helper()
	if res := c.login("/admin/login", "root", "supersecret"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("the superadmin sign-in answered %d", res.StatusCode)
	}
}
