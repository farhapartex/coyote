package cli

import (
	"io"
	"os"
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
