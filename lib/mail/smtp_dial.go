package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

func (s *SMTPSender) dial(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: s.options.Timeout}

	if s.options.TLS == TLSImplicit {
		conn, err := (&tls.Dialer{
			NetDialer: dialer,
			Config:    s.options.tlsConfig(),
		}).DialContext(ctx, "tcp", s.options.addr())
		if err != nil {
			return nil, fmt.Errorf("%w: dialing %s over TLS: %w", ErrSendFailed, s.options.addr(), err)
		}
		return conn, nil
	}

	conn, err := dialer.DialContext(ctx, "tcp", s.options.addr())
	if err != nil {
		return nil, fmt.Errorf("%w: dialing %s: %w", ErrSendFailed, s.options.addr(), err)
	}
	return conn, nil
}

func (s *SMTPSender) deadline(ctx context.Context, conn net.Conn) error {
	when := time.Now().Add(s.options.Timeout)
	if fromContext, ok := ctx.Deadline(); ok && fromContext.Before(when) {
		when = fromContext
	}
	if err := conn.SetDeadline(when); err != nil {
		return fmt.Errorf("%w: setting a deadline: %w", ErrSendFailed, err)
	}
	return nil
}

func watchContext(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	return func() { close(done) }
}
