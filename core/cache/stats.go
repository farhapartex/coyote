package cache

type Stats struct {
	Alias     string
	Backend   string
	Hits      int64
	Misses    int64
	Sets      int64
	Deletes   int64
	Errors    int64
	Evictions int64
	Entries   int
}

func (s Stats) Lookups() int64 { return s.Hits + s.Misses }

func (s Stats) HitRate() float64 {
	total := s.Lookups()
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

type Status struct {
	Alias     string
	Backend   string
	Address   string
	Reachable bool
	Err       error
}
