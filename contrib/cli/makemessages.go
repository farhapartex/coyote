package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/farhapartex/coyote/contrib/messages"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/settings"
)

const NameMakeMessages = "makemessages"

type MakeMessages struct {
	Locale string
}

func (MakeMessages) Name() string { return NameMakeMessages }

func (MakeMessages) Summary() string {
	return "extract translatable text from your templates and Go source"
}

func MakeMessagesFromEnv() MakeMessages {
	return MakeMessages{Locale: flagSet(EnvLocale)}
}

func (m MakeMessages) Run(ctx Context) error {
	cfg := ctx.App.Config()
	dir := localeDir(cfg)

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	scan, err := messages.Walk(root)
	if err != nil {
		return err
	}
	found := scan.Messages.All()

	fmt.Fprintf(ctx.Out, "scanned    %d file(s)\n", scan.Files)
	fmt.Fprintf(ctx.Out, "extracted  %d message(s)\n", len(found))
	fmt.Fprintf(ctx.Out, "catalogs   %s\n\n", dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	template := filepath.Join(dir, i18n.DefaultTemplate)
	if err := os.WriteFile(template, []byte(messages.RenderTemplate(cfg.Admin.SiteName, found)), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "wrote %s\n", filepath.Base(template))

	for _, tag := range m.targets(cfg) {
		if err := m.mergeOne(ctx, dir, tag, found); err != nil {
			return err
		}
	}

	if len(scan.Problems) > 0 {
		fmt.Fprintf(ctx.Out, "\n%d message(s) could not be extracted:\n", len(scan.Problems))
		for _, problem := range scan.Problems {
			fmt.Fprintf(ctx.Out, "  ! %s\n", problem)
		}
	}
	return nil
}

func (m MakeMessages) targets(cfg settings.Settings) []string {
	if m.Locale != "" {
		return []string{m.Locale}
	}
	out := []string{}
	for _, tag := range cfg.I18N.Supported {
		if i18n.Normalise(tag) == i18n.Normalise(cfg.I18N.Locale()) {
			continue
		}
		out = append(out, tag)
	}
	return out
}

func (m MakeMessages) mergeOne(ctx Context, dir, tag string, found []messages.Message) error {
	path := filepath.Join(dir, i18n.Normalise(tag)+i18n.Extension)

	var existing i18n.Catalog
	if handle, err := os.Open(path); err == nil {
		parsed, parseErr := i18n.ParsePO(tag, handle)
		handle.Close()
		if parseErr != nil {
			return fmt.Errorf("coyote/cli: %s: %w", path, parseErr)
		}
		existing = parsed
	}

	document, report := messages.Merge(existing, found, messages.PluralsOf(existing))
	body := document.Render(ctx.App.Config().Admin.SiteName, i18n.Normalise(tag))

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "merged %-12s %s\n", filepath.Base(path), report.Summary())
	return nil
}
