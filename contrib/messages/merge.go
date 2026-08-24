package messages

import (
	"strconv"

	"github.com/farhapartex/coyote/core/i18n"
)

type MergeReport struct {
	Added    int
	Kept     int
	Obsolete int
	Fuzzy    int
}

func Merge(existing i18n.Catalog, found []Message, plurals int) (Document, MergeReport) {
	document := Document{}
	report := MergeReport{}

	previous := map[string]i18n.Entry{}
	if existing != nil {
		for _, entry := range i18n.Entries(existing) {
			if entry.Header {
				document.Header = entry
				continue
			}
			previous[entry.Key()] = entry
		}
	}
	if plurals < 1 {
		plurals = 2
	}

	seen := map[string]bool{}
	for _, message := range found {
		key := message.Key()
		seen[key] = true

		entry := i18n.Entry{
			Context:  message.Context,
			Singular: message.Singular,
			Plural:   message.Plural,
		}
		before, had := previous[key]
		if had {
			entry.Forms = before.Forms
			entry.Fuzzy = before.Fuzzy
			report.Kept++
			if before.Fuzzy {
				report.Fuzzy++
			}
		} else {
			report.Added++
		}
		entry.Forms = fitForms(entry.Forms, message.Plural != "", plurals)
		document.Entries = append(document.Entries, entry)
	}

	for _, entry := range i18n.Entries(existing) {
		if entry.Header || seen[entry.Key()] {
			continue
		}
		if !entry.Translated() {
			continue
		}
		document.Obsolete = append(document.Obsolete, entry)
		report.Obsolete++
	}
	return document, report
}

func fitForms(forms []string, plural bool, plurals int) []string {
	wanted := 1
	if plural {
		wanted = plurals
	}
	out := make([]string, wanted)
	copy(out, forms)
	return out
}

func PluralsOf(existing i18n.Catalog) int {
	if existing == nil {
		return 2
	}
	if header := existing.Header("plural-forms"); header != "" {
		if rule, err := i18n.ParsePluralForms(header); err == nil {
			return rule.Forms()
		}
	}
	return existing.Plurals()
}

func (r MergeReport) Summary() string {
	return strconv.Itoa(r.Added) + " new, " +
		strconv.Itoa(r.Kept) + " kept, " +
		strconv.Itoa(r.Fuzzy) + " fuzzy, " +
		strconv.Itoa(r.Obsolete) + " obsolete"
}
