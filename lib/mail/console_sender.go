package mail

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

type ConsoleSender struct {
	writer  io.Writer
	encoder Encoder
	mu      sync.Mutex
}

func NewConsole(w io.Writer) *ConsoleSender {
	if w == nil {
		w = os.Stdout
	}
	return &ConsoleSender{writer: w, encoder: NewEncoder()}
}

func (c *ConsoleSender) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	raw, err := c.encoder.Encode(message)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := fmt.Fprintf(c.writer, "%s%s\r\n", consoleRule, raw); err != nil {
		return fmt.Errorf("%w: writing to the console: %w", ErrSendFailed, err)
	}
	return nil
}

const consoleRule = "-------------------------------- coyote/mail --------------------------------\r\n"
