package cli

import (
	"context"
	"io"
	"os"
	"strings"
)

type Context struct {
	App Application
	Out io.Writer
	In  io.Reader
}

func (c Context) Input() io.Reader {
	if c.In == nil {
		return os.Stdin
	}
	return c.In
}

type Command interface {
	Name() string
	Summary() string
	Run(Context) error
}

func Args() []string {
	raw := strings.TrimSpace(os.Getenv(EnvArgs))
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

func (c Context) Context() context.Context { return context.Background() }
