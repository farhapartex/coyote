package messages

import (
	"fmt"
	"strings"

	"github.com/farhapartex/coyote/core/i18n"
)

const templateHeader = `msgid ""
msgstr ""
"Project-Id-Version: %s\n"
"Language: %s\n"
"MIME-Version: 1.0\n"
"Content-Type: text/plain; charset=UTF-8\n"
"Content-Transfer-Encoding: 8bit\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\n"
`

type Document struct {
	Header   i18n.Entry
	Entries  []i18n.Entry
	Obsolete []i18n.Entry
}

func (d Document) Render(project, tag string) string {
	var b strings.Builder

	if len(d.Header.Forms) > 0 && d.Header.Forms[0] != "" {
		b.WriteString("msgid \"\"\nmsgstr \"\"\n")
		for _, line := range strings.Split(strings.TrimRight(d.Header.Forms[0], "\n"), "\n") {
			fmt.Fprintf(&b, "%s\n", quote(line+"\n"))
		}
	} else {
		fmt.Fprintf(&b, templateHeader, project, tag)
	}

	for _, entry := range d.Entries {
		b.WriteString("\n")
		writeEntry(&b, entry, "")
	}
	for _, entry := range d.Obsolete {
		b.WriteString("\n")
		writeEntry(&b, entry, "#~ ")
	}
	return b.String()
}

func writeEntry(b *strings.Builder, entry i18n.Entry, prefix string) {
	if entry.Fuzzy {
		b.WriteString("#, fuzzy\n")
	}
	if entry.Context != "" {
		fmt.Fprintf(b, "%smsgctxt %s\n", prefix, quote(entry.Context))
	}
	fmt.Fprintf(b, "%smsgid %s\n", prefix, quote(entry.Singular))

	if entry.Plural == "" {
		form := ""
		if len(entry.Forms) > 0 {
			form = entry.Forms[0]
		}
		fmt.Fprintf(b, "%smsgstr %s\n", prefix, quote(form))
		return
	}

	fmt.Fprintf(b, "%smsgid_plural %s\n", prefix, quote(entry.Plural))
	forms := entry.Forms
	if len(forms) == 0 {
		forms = []string{"", ""}
	}
	for index, form := range forms {
		fmt.Fprintf(b, "%smsgstr[%d] %s\n", prefix, index, quote(form))
	}
}

func RenderTemplate(project string, found []Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, templateHeader, project, "en")

	for _, message := range found {
		b.WriteString("\n")
		for _, reference := range message.References {
			fmt.Fprintf(&b, "#: %s\n", reference)
		}
		if message.Context != "" {
			fmt.Fprintf(&b, "msgctxt %s\n", quote(message.Context))
		}
		fmt.Fprintf(&b, "msgid %s\n", quote(message.Singular))
		if message.Plural == "" {
			b.WriteString("msgstr \"\"\n")
			continue
		}
		fmt.Fprintf(&b, "msgid_plural %s\n", quote(message.Plural))
		b.WriteString("msgstr[0] \"\"\nmsgstr[1] \"\"\n")
	}
	return b.String()
}

func quote(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\t", `\t`,
		"\r", `\r`,
	)
	return `"` + replacer.Replace(value) + `"`
}
