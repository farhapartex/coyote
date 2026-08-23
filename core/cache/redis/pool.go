package redis

import (
	"context"
	"sync"

	"github.com/farhapartex/coyote/core/cache"
)

type pool struct {
	opts   Options
	idle   chan *conn
	tokens chan struct{}
	mu     sync.Mutex
	closed bool
}

func newPool(opts Options) *pool {
	p := &pool{
		opts:   opts,
		idle:   make(chan *conn, opts.PoolSize),
		tokens: make(chan struct{}, opts.PoolSize),
	}
	for range opts.PoolSize {
		p.tokens <- struct{}{}
	}
	return p
}

func (p *pool) acquire(ctx context.Context) (*conn, error) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, cache.ErrClosed
	}

	select {
	case <-p.tokens:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case existing := <-p.idle:
		return existing, nil
	default:
	}

	fresh, err := dial(ctx, p.opts)
	if err != nil {
		p.tokens <- struct{}{}
		return nil, err
	}
	return fresh, nil
}

func (p *pool) release(c *conn) {
	defer func() { p.tokens <- struct{}{} }()
	if c == nil {
		return
	}

	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed || c.broken {
		c.close()
		return
	}

	select {
	case p.idle <- c:
	default:
		c.close()
	}
}

func (p *pool) close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	for {
		select {
		case existing := <-p.idle:
			existing.close()
		default:
			return nil
		}
	}
}
