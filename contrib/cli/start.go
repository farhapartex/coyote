package cli

type Start struct{}

func (Start) Name() string { return NameStart }

func (Start) Summary() string { return "build and run the project in the current directory" }

func (Start) Run(ctx Context) error {
	return ctx.App.Serve()
}
