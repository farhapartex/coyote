package jobs

type State string

const (
	Queued  State = "queued"
	Running State = "running"
	Done    State = "done"
	Dead    State = "dead"
)

func States() []State {
	return []State{Queued, Running, Done, Dead}
}

func (s State) Terminal() bool {
	return s == Done || s == Dead
}
