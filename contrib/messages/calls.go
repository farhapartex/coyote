package messages

type shape struct {
	context  int
	singular int
	plural   int
	literals int
}

var known = map[string]shape{
	"T":   {context: -1, singular: 0, plural: -1, literals: 1},
	"Tf":  {context: -1, singular: 0, plural: -1, literals: 1},
	"TC":  {context: 0, singular: 1, plural: -1, literals: 2},
	"TCf": {context: 0, singular: 1, plural: -1, literals: 2},
	"N":   {context: -1, singular: 0, plural: 1, literals: 2},
	"NC":  {context: 0, singular: 1, plural: 2, literals: 3},
}

func shapeOf(name string) (shape, bool) {
	found, ok := known[name]
	return found, ok
}

func (s shape) build(literals []string) (Message, bool) {
	if len(literals) < s.literals {
		return Message{}, false
	}
	out := Message{Singular: literals[s.singular]}
	if s.context >= 0 {
		out.Context = literals[s.context]
	}
	if s.plural >= 0 {
		out.Plural = literals[s.plural]
	}
	if out.Singular == "" {
		return Message{}, false
	}
	return out, true
}
