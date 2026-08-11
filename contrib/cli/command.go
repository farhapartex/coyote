package cli

import "io"

type Context struct {
	App Application
	Out io.Writer
}

type Command interface {
	Name() string
	Summary() string
	Run(Context) error
}
