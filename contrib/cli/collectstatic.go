package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/farhapartex/coyote/contrib/collect"
)

type CollectStatic struct{}

func (CollectStatic) Name() string { return NameCollectStatic }

func (CollectStatic) Summary() string { return "fingerprint static files and write a manifest" }

func (CollectStatic) Run(ctx Context) error {
	cfg := ctx.App.Config()

	source := cfg.Static.FS
	if source == nil && cfg.Static.Dir == "" {
		return errors.New("coyote/cli: no static files configured; set Static.FS or Static.Dir")
	}

	target := cfg.Path("staticfiles")
	if source == nil {
		return errors.New("coyote/cli: Static.FS is required to collect; a plain Static.Dir is already served as-is")
	}

	result, err := collect.Run(source, target)
	if err != nil {
		return err
	}

	fmt.Fprintf(ctx.Out, "collected %d file(s) into %s\n\n", len(result.Entries), filepath.Clean(target))
	for _, name := range result.Names() {
		fmt.Fprintf(ctx.Out, "  %s -> %s\n", name, result.Entries[name])
	}
	fmt.Fprintf(ctx.Out, "\nwrote %s\n", collect.ManifestName)
	return nil
}
