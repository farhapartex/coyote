# 36. Background jobs

[← Back to contents](README.md)

Work that should not happen inside a request — sending email, rendering a report, rebuilding a
thumbnail — goes on a queue and runs in a worker. The queue is a table in your own database, so there
is no broker to install and nothing new in `go.mod`.

Jobs are off until you ask for them.

```go
s.Jobs.Enabled = true
```

That registers the `jobs` table, so run `coyote makemigrations && coyote migrate` after turning it
on. Nothing else changes: no goroutine starts, the admin portal grows no section, and validation
demands nothing more of your settings.

## Declaring a job

A job is a name and a function. The function's argument is its payload, and the framework encodes and
decodes it as JSON for you.

```go
package main

import (
	"context"

	"github.com/farhapartex/coyote/core/jobs"
)

type WelcomeArgs struct {
	UserID string `json:"user_id"`
}

func init() {
	jobs.Handle("email.welcome", sendWelcome)
}

func sendWelcome(ctx context.Context, args WelcomeArgs) error {
	user, err := users.ByID(ctx, args.UserID)
	if err != nil {
		return err
	}
	return mailer.Send(ctx, welcomeMessage(user))
}
```

Register in `init()` so a worker process has every handler the moment it starts. `jobs.Handle` panics
on a duplicate name, which fails at startup rather than silently running the wrong function.

A job with no arguments declares `struct{}` and is enqueued with `nil`:

```go
jobs.Handle("cache.warm", func(ctx context.Context, _ struct{}) error {
	return warmTheCache(ctx)
})
```

## Enqueueing

```go
id, err := jobs.Enqueue(r.Context(), a.Queue(), "email.welcome", WelcomeArgs{UserID: user.ID})
```

`a.Queue()` is `nil` while jobs are off, and enqueueing to it returns `jobs.ErrNoQueue` rather than
panicking.

Options ride along as a final argument:

```go
jobs.Enqueue(ctx, a.Queue(), "report.build", args, jobs.Options{
	Queue:       "reports",
	Priority:    5,
	Delay:       10 * time.Minute,
	MaxAttempts: 10,
	Fingerprint: "report:" + month,
})
```

| Option | Effect |
| --- | --- |
| `Queue` | Which queue the job waits on; defaults to `default` |
| `Priority` | Higher goes first. Ties break on the older job |
| `Delay` | Wait this long before the job becomes due |
| `MaxAttempts` | Overrides `Jobs.MaxAttempts`; `jobs.Forever` never gives up |
| `Fingerprint` | Refuses a second job carrying the same one — see [Only once](#only-once) |

## A job and its data commit together

This is the reason the queue is a table. `app.Tx` carries a queue bound to the transaction, so the
job row is inserted alongside the data the job is about:

```go
err := a.Transaction(ctx, func(tx *app.Tx) error {
	id, err := tx.Store().Insert(ctx, schema, record)
	if err != nil {
		return err
	}
	return enqueueRender(ctx, tx.Queue(), id)
})
```

If the transaction rolls back, the job goes with it. A job can never reference a row that was never
committed, and a committed row can never be missing its job. No broker can promise that without an
outbox table — at which point the database is the queue anyway.

## Running workers

Two ways, one implementation.

In the web process, for development and small deployments:

```go
s.Jobs.Enabled = true
s.Jobs.Workers = 4
```

As its own process, which is what you want in production:

```
coyote worker
coyote worker --concurrency=8 --queues=email,reports
```

`Jobs.Workers` is `0` by default, so a web process never becomes a worker by accident. Both paths run
the same pool, and both drain on `SIGINT` and `SIGTERM`: the pool stops claiming, in-flight jobs get
`Jobs.DrainTimeout` to finish, and anything still running past that is cancelled through its
`context` and returns to the queue for another worker.

## Retries

A handler that returns an error is retried with exponential backoff and jitter, from `Jobs.Backoff`
up to `Jobs.BackoffCeiling`. The jitter stops a batch of simultaneous failures from retrying in
lockstep.

When a job runs out of attempts it becomes **dead**: the row stays, carrying its last error, visible
and retryable in the admin portal. Dead jobs are never swept automatically — a silently dropped job
is worse than a full table.

Two failures skip the retries entirely and go straight to dead, because no amount of waiting would
help:

- **No handler is registered for that name.** Usually a job type that was renamed while rows were
  still queued.
- **The payload no longer decodes** into the handler's argument. Usually a field that changed type.

A handler that panics is recovered, logged with its stack, and counts as one failed attempt. One bad
job cannot take a worker down.

## Only once

Set a `Fingerprint` and a second job carrying the same one is refused with
`jobs.ErrDuplicateFingerprint`. It is a unique index, so the guarantee holds across every process
sharing the database — no lock, no leader election.

A fingerprint you leave unset is stored as `NULL`, and any number of rows may have none.

## Periodic jobs

```go
jobs.Every(6*time.Hour, "cache.warm", nil)
```

The scheduler truncates the clock to the interval and enqueues with a fingerprint built from the name
and that slot, so several worker processes sharing a database still produce exactly one job per
interval. The one that wins the unique index enqueues it; the others see the duplicate and move on.

Intervals only — there is no cron syntax. `jobs.Every(24*time.Hour, …)` means once a day, not 3am.

Two periodic jobs come with the framework and need nothing from you. They are the ones this guide
used to tell you to schedule yourself:

| Job | Every | What it does |
| --- | --- | --- |
| `coyote.uploads.sweep` | 6 hours | Deletes abandoned staged uploads and expired trash; see [Uploads](31-uploads.md) |
| `coyote.tokens.sweep` | 24 hours | Deletes expired password reset tokens; see [Authentication](14-authentication.md) |

Each is registered only when that feature is on, and scheduling either one yourself replaces the
built-in interval.

## In the admin portal

With jobs on, the portal grows a **Jobs** page: counts by state, a filter per state, the last error
on each row, and retry and delete as single-row and bulk actions. It is superadmin-only, like the
sessions page, because a payload can hold anything.

Retry is refused on a job that has not finished — retrying a running job would duplicate the work.

## Settings

| Setting | Default | Notes |
| --- | --- | --- |
| `Jobs.Enabled` | `false` | Off means no table, no goroutine, no admin page |
| `Jobs.Queue` | none | Supply your own `jobs.Queue` instead of the database one |
| `Jobs.Queues` | `["default"]` | Which queues a worker takes from |
| `Jobs.Workers` | `0` | Workers inside the web process; `0` runs none |
| `Jobs.MaxAttempts` | `3` | `jobs.Forever` retries without a ceiling |
| `Jobs.Backoff` | `10s` | How long the first retry waits |
| `Jobs.BackoffCeiling` | `1h` | The longest any retry waits |
| `Jobs.ClaimTimeout` | `5m` | A job silent this long is reclaimed from its worker |
| `Jobs.DrainTimeout` | `30s` | How long a worker gets to finish on shutdown |
| `Jobs.PollInterval` | `1s` | How often a worker looks for work |
| `Jobs.DoneTTL` | `24h` | Finished jobs are deleted after this; `0` keeps them |

Validation refuses combinations that cannot work: `Jobs.ClaimTimeout` shorter than
`Jobs.DrainTimeout` would let a job still finishing during shutdown be reclaimed and run twice, and a
`Jobs.PollInterval` under 100ms is a busy loop against your database.

## How a job is claimed

Worth knowing, because it sets the limits below. A worker selects a batch of due rows, then claims one
with a single conditional update:

```sql
UPDATE jobs SET state = 'running', locked_by = ?, attempts = attempts + 1
WHERE id = ? AND state = 'queued'
```

One row changed means the job is yours; none means another worker got there first, and you try the
next candidate. That statement is atomic on SQLite, PostgreSQL and MySQL alike, which is why the
queue needs no `SELECT … FOR UPDATE SKIP LOCKED` — SQLite has neither.

While a job runs, its worker quietly pushes the lock forward. A worker that dies stops doing so, and
after `Jobs.ClaimTimeout` another worker takes the job back. If its attempts are already used up it
goes to dead instead.

## Limits

Say these out loud rather than discover them:

- **Latency is bounded by `Jobs.PollInterval`.** A job enqueued now starts within about a second, not
  within a millisecond. Fine for email and thumbnails; wrong for anything a user is watching.
- **Under contention a worker can come back empty** while work is waiting, having lost the race on
  every candidate it looked at. It tries again on the next poll.
- **A shared database as a queue has a ceiling.** Comfortable into the low thousands of jobs a
  minute. Past that it wants `SKIP LOCKED`, then a real broker. `jobs.Queue` is an interface, so
  neither is a rewrite.
- **Anything writing to the `jobs` table outside the framework must use UTC.** SQLite stores
  timestamps as text and compares them lexically, so a row written with a local-time offset stops
  matching the claim and recover predicates.
- **Counters are per process.** Two worker processes do not share their rate limits or statistics.

## Supplying your own queue

`jobs.Queue` is the seam. Implement it and set `Jobs.Queue`, and the runner, the scheduler, the
worker command and the admin page all use it instead of the table.

```go
type Queue interface {
	Enqueue(ctx context.Context, job Job) (string, error)
	Claim(ctx context.Context, worker string, queues []string, limit int) ([]Record, error)
	Complete(ctx context.Context, id string, at time.Time) error
	Fail(ctx context.Context, id string, cause string, retryAt *time.Time) error
	Retry(ctx context.Context, id string, at time.Time) error
	Heartbeat(ctx context.Context, id string, at time.Time) error
	Recover(ctx context.Context, lockedBefore time.Time) (int, error)
	Sweep(ctx context.Context, finishedBefore time.Time) (int, error)
	Stats(ctx context.Context) (Stats, error)
	WithTx(tx *gorm.DB) Queue
}
```

A backend with no transactions returns itself from `WithTx`, and gives up the guarantee in [A job and
its data commit together](#a-job-and-its-data-commit-together).

## Testing your jobs

A handler is a plain function, so test it as one. For the queue, build a throwaway app the way
[Testing](26-testing.md) describes and drive it directly:

```go
id, err := jobs.Enqueue(t.Context(), a.Queue(), "email.welcome", args)
claimed, err := a.Queue().Claim(t.Context(), "test", nil, 1)
err = a.Queue().Complete(t.Context(), id, time.Now())
```

No worker, no sleeping, no timers.

## Next

[Architecture →](27-architecture.md)
