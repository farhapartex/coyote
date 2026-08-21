package cache

import "sync"

type call struct {
	done  chan struct{}
	value []byte
	err   error
}

type flight struct {
	mu      sync.Mutex
	pending map[string]*call
}

func newFlight() *flight {
	return &flight{pending: make(map[string]*call)}
}

func (f *flight) do(key string, build func() ([]byte, error)) ([]byte, error) {
	f.mu.Lock()
	if existing, found := f.pending[key]; found {
		f.mu.Unlock()
		<-existing.done
		return existing.value, existing.err
	}
	current := &call{done: make(chan struct{}), err: ErrBuildAbandoned}
	f.pending[key] = current
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.pending, key)
		f.mu.Unlock()
		close(current.done)
	}()

	value, err := build()
	current.value, current.err = value, err
	return value, err
}
