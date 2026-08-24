package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/farhapartex/coyote/contrib/messages"
	"github.com/farhapartex/coyote/core/i18n"
	"github.com/farhapartex/coyote/core/settings"
)

const NameCheckMessages = "checkmessages"

var ErrIncompleteCatalog = errors.New("coyote/cli: translations are incomplete")

type CheckMessages struct {
	Locale string
	Strict bool
}

func (CheckMessages) Name() string { return NameCheckMessages }

func (CheckMessages) Summary() string {
	return "report missing, fuzzy or obsolete translations"
}

func CheckMessagesFromEnv() CheckMessages {
	return CheckMessages{Locale: flagSet(EnvLocale), Strict: flagBool(EnvStrict)}
}

type localeReport struct {
	tag        string
	total      int
	translated int
	fuzzy      []string
	missing    []string
	obsolete   []string
	absent     bool
}

func (r localeReport) percent() int {
	if r.total == 0 {
		return 100
	}
	return r.translated * 100 / r.total
}

func (c CheckMessages) Run(ctx Context) error {
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

	fmt.Fprintf(ctx.Out, "extracted  %d message(s) from %d file(s)\n", len(found), scan.Files)
	fmt.Fprintf(ctx.Out, "catalogs   %s\n\n", dir)

	incomplete := false
	for _, tag := range c.targets(cfg) {
		report := c.inspect(dir, tag, found)
		c.print(ctx, report)
		if len(report.missing) > 0 || report.absent {
			incomplete = true
		}
		if c.Strict && (len(report.fuzzy) > 0 || len(report.obsolete) > 0) {
			incomplete = true
		}
	}

	if len(scan.Problems) > 0 {
		fmt.Fprintf(ctx.Out, "%d message(s) could not be extracted:\n", len(scan.Problems))
		for _, problem := range scan.Problems {
			fmt.Fprintf(ctx.Out, "  ! %s\n", problem)
		}
		if c.Strict {
			incomplete = true
		}
	}

	if incomplete {
		return ErrIncompleteCatalog
	}
	fmt.Fprintln(ctx.Out, "every catalog is complete")
	return nil
}

func (c CheckMessages) targets(cfg settings.Settings) []string {
	if c.Locale != "" {
		return []string{c.Locale}
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

func (c CheckMessages) inspect(dir, tag string, found []messages.Message) localeReport {
	report := localeReport{tag: i18n.Normalise(tag), total: len(found)}

	path := filepath.Join(dir, report.tag+i18n.Extension)
	handle, err := os.Open(path)
	if err != nil {
		report.absent = true
		for _, message := range found {
			report.missing = append(report.missing, message.Singular)
		}
		return report
	}
	defer handle.Close()

	catalog, err := i18n.ParsePO(tag, handle)
	if err != nil {
		report.absent = true
		return report
	}

	entries := map[string]i18n.Entry{}
	for _, entry := range i18n.Entries(catalog) {
		if !entry.Header {
			entries[entry.Key()] = entry
		}
	}

	wanted := map[string]bool{}
	for _, message := range found {
		wanted[message.Key()] = true
		entry, present := entries[message.Key()]
		switch {
		case !present || len(entry.Forms) == 0 || entry.Forms[0] == "":
			report.missing = append(report.missing, message.Singular)
		case entry.Fuzzy:
			report.fuzzy = append(report.fuzzy, message.Singular)
		default:
			report.translated++
		}
	}
	for key, entry := range entries {
		if !wanted[key] {
			report.obsolete = append(report.obsolete, entry.Singular)
		}
	}

	sort.Strings(report.missing)
	sort.Strings(report.fuzzy)
	sort.Strings(report.obsolete)
	return report
}

func (c CheckMessages) print(ctx Context, report localeReport) {
	if report.absent {
		fmt.Fprintf(ctx.Out, "%-8s no catalog; %d message(s) untranslated\n", report.tag, len(report.missing))
		return
	}

	fmt.Fprintf(ctx.Out, "%-8s %d%% translated (%d of %d)\n",
		report.tag, report.percent(), report.translated, report.total)

	for label, list := range map[string][]string{
		"untranslated": report.missing,
		"fuzzy":        report.fuzzy,
		"obsolete":     report.obsolete,
	} {
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(ctx.Out, "  %d %s\n", len(list), label)
		for _, entry := range list {
			fmt.Fprintf(ctx.Out, "    - %s\n", entry)
		}
	}
}

func localeDir(cfg settings.Settings) string {
	dir := cfg.I18N.Dir
	if dir == "" {
		dir = i18n.DefaultDir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return cfg.Path(dir)
}
