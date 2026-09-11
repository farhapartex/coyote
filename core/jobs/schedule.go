package jobs

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Periodic struct {
	Kind     string
	Every    time.Duration
	Args     any
	Queue    string
	Priority int
}

func (p Periodic) Valid() error {
	if p.Kind == "" {
		return ErrKindMissing
	}
	if p.Every <= 0 {
		return fmt.Errorf("%w: %s", ErrIntervalMissing, p.Kind)
	}
	return nil
}

func (p Periodic) Slot(now time.Time) time.Time {
	return now.UTC().Truncate(p.Every)
}

func (p Periodic) Fingerprint(slot time.Time) string {
	return p.Kind + "@" + slot.Format(time.RFC3339)
}

type Schedule struct {
	mu      sync.RWMutex
	entries map[string]Periodic
	parent  *Schedule
}

func NewSchedule() *Schedule {
	return &Schedule{entries: map[string]Periodic{}}
}

func ScheduleUnder(parent *Schedule) *Schedule {
	return &Schedule{entries: map[string]Periodic{}, parent: parent}
}

func (s *Schedule) Add(entry Periodic) error {
	if err := entry.Valid(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.entries[entry.Kind]; taken {
		return fmt.Errorf("%w: %s", ErrDuplicateSchedule, entry.Kind)
	}
	if s.parent != nil && s.parent.Holds(entry.Kind) {
		return fmt.Errorf("%w: %s", ErrDuplicateSchedule, entry.Kind)
	}
	s.entries[entry.Kind] = entry
	return nil
}

func (s *Schedule) Holds(kind string) bool {
	s.mu.RLock()
	_, found := s.entries[kind]
	s.mu.RUnlock()
	if found {
		return true
	}
	return s.parent != nil && s.parent.Holds(kind)
}

func (s *Schedule) Entries() []Periodic {
	merged := map[string]Periodic{}
	if s.parent != nil {
		for _, entry := range s.parent.Entries() {
			merged[entry.Kind] = entry
		}
	}
	s.mu.RLock()
	for kind, entry := range s.entries {
		merged[kind] = entry
	}
	s.mu.RUnlock()

	out := make([]Periodic, 0, len(merged))
	for _, entry := range merged {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

func (s *Schedule) Len() int { return len(s.Entries()) }
