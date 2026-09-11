package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/jobs"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
	"github.com/farhapartex/coyote/lib/text"
)

const (
	RetryAction  = "retry"
	jobErrorPeek = 160
)

type jobRow struct {
	ID        string
	Short     string
	Kind      string
	Queue     string
	State     string
	Attempts  string
	RunAt     time.Time
	Error     string
	Retryable bool
}

func (a *Admin) jobsEnabled() bool {
	return a.app.Settings.Jobs.Enabled && a.app.Queue() != nil
}

func (a *Admin) jobList(w http.ResponseWriter, r *http.Request) {
	if !a.jobsEnabled() {
		a.notFound(w, r)
		return
	}
	records, ok := a.storeFor(w, r)
	if !ok {
		return
	}
	schema, err := a.app.Describe(jobs.Record{})
	if err != nil {
		a.fail(w, r, err)
		return
	}

	perPage := a.app.Settings.Pagination.PerPage
	number := 1
	if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 1 {
		number = n
	}
	offset := 0
	if perPage > 0 && number > 1 {
		offset = (number - 1) * perPage
	}

	state := r.URL.Query().Get("state")
	query := model.Query{Sort: "-run_at", Limit: perPage, Offset: offset}
	if wanted, known := knownState(state); known {
		query.Filters = append(query.Filters, model.Filter{Column: "state", Op: model.Eq, Value: string(wanted)})
	}

	result, err := records.List(r.Context(), schema, query)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	page := view.Paginate(a.app.Settings.Pagination.Paginator, result.Total, number, perPage)
	if page.Offset != offset {
		query.Limit, query.Offset = page.Limit, page.Offset
		if result, err = records.List(r.Context(), schema, query); err != nil {
			a.fail(w, r, err)
			return
		}
	}

	stats, err := a.app.Queue().Stats(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}

	a.render(w, r, http.StatusOK, "jobs.html", view.Data{
		"Nav":    "jobs",
		"Jobs":   jobRows(result.Records),
		"Total":  result.Total,
		"Page":   page,
		"State":  state,
		"States": jobs.States(),
		"Stats":  stats,
	})
}

func jobRows(records []model.Record) []jobRow {
	out := make([]jobRow, 0, len(records))
	for _, record := range records {
		state := record.String("state")
		out = append(out, jobRow{
			ID:        record.String("id"),
			Short:     text.Truncate(record.String("id"), 8),
			Kind:      record.String("kind"),
			Queue:     record.String("queue"),
			State:     state,
			Attempts:  record.String("attempts") + "/" + attemptCeiling(record),
			RunAt:     runAt(record),
			Error:     text.Truncate(record.String("last_error"), jobErrorPeek),
			Retryable: state == string(jobs.Dead),
		})
	}
	return out
}

func attemptCeiling(record model.Record) string {
	if record.String("max_attempts") == strconv.Itoa(jobs.Forever) {
		return "∞"
	}
	return record.String("max_attempts")
}

func runAt(record model.Record) time.Time {
	if stamp, ok := record.Get("run_at").(time.Time); ok {
		return stamp
	}
	return time.Time{}
}

func knownState(candidate string) (jobs.State, bool) {
	for _, state := range jobs.States() {
		if string(state) == candidate {
			return state, true
		}
	}
	return "", false
}

func (a *Admin) jobRetry(w http.ResponseWriter, r *http.Request) {
	if !a.jobsEnabled() {
		a.notFound(w, r)
		return
	}
	back := a.prefix + "/jobs"
	if err := a.requeue(r, r.PathValue("id")); err != nil {
		view.Error(r, humanize(r.Context(), err))
		view.Redirect(w, r, back)
		return
	}
	view.Success(r, i18n.T(r.Context(), "Job queued to run again."))
	view.Redirect(w, r, back)
}

func (a *Admin) requeue(r *http.Request, id string) error {
	return a.app.Queue().Retry(r.Context(), id, time.Now())
}
