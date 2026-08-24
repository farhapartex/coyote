package messages

import (
	"sort"
	"strconv"
)

type Message struct {
	Context    string
	Singular   string
	Plural     string
	References []string
}

func (m Message) Key() string {
	if m.Context == "" {
		return m.Singular
	}
	return m.Context + "\x04" + m.Singular
}

type Problem struct {
	File   string
	Line   int
	Reason string
}

func (p Problem) String() string {
	return p.File + ":" + strconv.Itoa(p.Line) + ": " + p.Reason
}

type Set struct {
	byKey map[string]*Message
	order []string
}

func NewSet() *Set {
	return &Set{byKey: map[string]*Message{}}
}

func (s *Set) Add(found Message) {
	key := found.Key()
	existing, seen := s.byKey[key]
	if !seen {
		copied := found
		s.byKey[key] = &copied
		s.order = append(s.order, key)
		return
	}
	if existing.Plural == "" {
		existing.Plural = found.Plural
	}
	existing.References = append(existing.References, found.References...)
}

func (s *Set) Len() int { return len(s.order) }

func (s *Set) All() []Message {
	out := make([]Message, 0, len(s.order))
	for _, key := range s.order {
		entry := *s.byKey[key]
		sort.Strings(entry.References)
		entry.References = unique(entry.References)
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Singular != out[j].Singular {
			return out[i].Singular < out[j].Singular
		}
		return out[i].Context < out[j].Context
	})
	return out
}

func (s *Set) Has(key string) bool {
	_, found := s.byKey[key]
	return found
}

func unique(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
