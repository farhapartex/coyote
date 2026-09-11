package jobs

import (
	"errors"
	"log/slog"
	"time"
)

func (r *Runner) tick() {
	defer r.running.Done()

	if r.schedules == nil || r.schedules.Len() == 0 {
		return
	}
	r.log.Info("jobs scheduling", slog.Int("periodic", r.schedules.Len()))

	for {
		r.enqueueDue(time.Now())
		if !r.wait(r.poll) {
			return
		}
	}
}

func (r *Runner) enqueueDue(now time.Time) {
	for _, entry := range r.schedules.Entries() {
		slot := entry.Slot(now)
		if !r.slotDue(entry.Kind, slot) {
			continue
		}

		_, err := Enqueue(r.ctx, r.queue, entry.Kind, entry.Args, Options{
			Queue:       entry.Queue,
			Priority:    entry.Priority,
			Fingerprint: entry.Fingerprint(slot),
		})
		switch {
		case err == nil, errors.Is(err, ErrDuplicateFingerprint):
			r.markSlot(entry.Kind, slot)
		default:
			if r.ctx.Err() == nil {
				r.log.Error("a periodic job could not be enqueued",
					slog.String("kind", entry.Kind), slog.Any("error", err))
			}
		}
	}
}

func (r *Runner) slotDue(kind string, slot time.Time) bool {
	r.slotMu.Lock()
	defer r.slotMu.Unlock()
	last, seen := r.slots[kind]
	return !seen || slot.After(last)
}

func (r *Runner) markSlot(kind string, slot time.Time) {
	r.slotMu.Lock()
	defer r.slotMu.Unlock()
	r.slots[kind] = slot
}
