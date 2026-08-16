package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
)

var ErrUnknownCommand = errors.New("coyote/cli: unknown command")

type Registry struct {
	commands map[string]Command
}

func NewRegistry(commands ...Command) *Registry {
	r := &Registry{commands: make(map[string]Command, len(commands))}
	r.Add(commands...)
	return r
}

func Default() *Registry {
	return NewRegistry(Start{}, Migrate{}, MakeMigrations{Label: os.Getenv(EnvName)}, SQLMigrate{}, CreateSuperadmin{}, SyncPermissions{}, CollectStatic{})
}

func (r *Registry) Add(commands ...Command) {
	for _, c := range commands {
		r.commands[c.Name()] = c
	}
}

func (r *Registry) Lookup(name string) (Command, bool) {
	c, ok := r.commands[name]
	return c, ok
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.commands))
	for name := range r.commands {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) Run(ctx Context, name string) error {
	if name == "" {
		name = NameStart
	}
	command, ok := r.Lookup(name)
	if !ok {
		return fmt.Errorf("%w: %q (known: %v)", ErrUnknownCommand, name, r.Names())
	}
	return command.Run(ctx)
}

func Dispatch(a Application) error {
	return Default().Run(Context{App: a, Out: os.Stdout, In: os.Stdin}, Requested())
}
