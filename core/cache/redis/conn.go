package redis

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"
)

type conn struct {
	raw    net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	opts   Options
	broken bool
}

func dial(ctx context.Context, opts Options) (*conn, error) {
	dialer := &net.Dialer{Timeout: opts.DialTimeout}

	var raw net.Conn
	var err error
	if opts.TLS != nil {
		raw, err = tls.DialWithDialer(dialer, "tcp", opts.Address, opts.TLS)
	} else {
		raw, err = dialer.DialContext(ctx, "tcp", opts.Address)
	}
	if err != nil {
		return nil, fmt.Errorf("coyote/cache/redis: dialling %s: %w", opts.Address, err)
	}

	c := &conn{
		raw:    raw,
		reader: bufio.NewReader(raw),
		writer: bufio.NewWriter(raw),
		opts:   opts,
	}
	if err := c.handshake(ctx); err != nil {
		c.close()
		return nil, err
	}
	return c, nil
}

func (c *conn) handshake(ctx context.Context) error {
	if c.opts.Password != "" {
		args := [][]byte{[]byte(c.opts.Password)}
		if c.opts.Username != "" {
			args = [][]byte{[]byte(c.opts.Username), []byte(c.opts.Password)}
		}
		if _, err := c.do(ctx, "AUTH", args...); err != nil {
			return err
		}
	}
	if c.opts.Database > 0 {
		if _, err := c.do(ctx, "SELECT", strconv.AppendInt(nil, int64(c.opts.Database), 10)); err != nil {
			return err
		}
	}
	return nil
}

func (c *conn) do(ctx context.Context, name string, args ...[]byte) (reply, error) {
	if err := c.deadline(ctx, c.opts.WriteTimeout); err != nil {
		return reply{}, err
	}
	if err := writeCommand(c.writer, name, args...); err != nil {
		c.broken = true
		return reply{}, fmt.Errorf("coyote/cache/redis: sending %s: %w", name, err)
	}

	if err := c.deadline(ctx, c.opts.ReadTimeout); err != nil {
		return reply{}, err
	}
	answer, err := readReply(c.reader)
	if err != nil {
		c.broken = true
		return reply{}, fmt.Errorf("coyote/cache/redis: reading the reply to %s: %w", name, err)
	}
	return answer, answer.err()
}

func (c *conn) deadline(ctx context.Context, timeout time.Duration) error {
	when := time.Now().Add(timeout)
	if fromContext, ok := ctx.Deadline(); ok && fromContext.Before(when) {
		when = fromContext
	}
	if err := c.raw.SetDeadline(when); err != nil {
		c.broken = true
		return err
	}
	return nil
}

func (c *conn) close() {
	if c.raw != nil {
		_ = c.raw.Close()
	}
}
