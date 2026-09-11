package jobs

import (
	"context"
	"fmt"
)

var registry = NewRegistry()

func Default() *Registry { return registry }

func Handle[T any](kind string, run func(context.Context, T) error) {
	if err := HandleOn(registry, kind, run); err != nil {
		panic(err)
	}
}

func HandleOn[T any](target *Registry, kind string, run func(context.Context, T) error) error {
	if run == nil {
		return fmt.Errorf("coyote/jobs: %s was registered with no function to run", kind)
	}
	return target.Add(kind, func(ctx context.Context, payload []byte) error {
		args, err := decodePayload[T](kind, payload)
		if err != nil {
			return err
		}
		return run(ctx, args)
	})
}
