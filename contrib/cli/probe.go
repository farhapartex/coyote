package cli

import (
	"errors"
	"fmt"
	"net"
	"time"
)

var ErrServerNotRunning = errors.New("coyote/cli: server is not running")

const probeTimeout = 2 * time.Second

func ServerRunning(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, probeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func RequireServer(addr string) error {
	if ServerRunning(addr) {
		return nil
	}
	return fmt.Errorf("%w at %s; start it first with: coyote start", ErrServerNotRunning, addr)
}
