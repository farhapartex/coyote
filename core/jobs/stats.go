package jobs

type Stats struct {
	Queued  int64
	Running int64
	Done    int64
	Dead    int64
}

func (s Stats) Total() int64 {
	return s.Queued + s.Running + s.Done + s.Dead
}

func (s Stats) Backlogged() bool {
	return s.Queued > 0
}
