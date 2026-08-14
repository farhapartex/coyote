package text

import "strings"

var acronyms = map[string]string{
	"id": "ID", "sku": "SKU", "url": "URL", "uri": "URI", "api": "API",
	"ip": "IP", "uuid": "UUID", "html": "HTML", "css": "CSS", "json": "JSON",
	"xml": "XML", "pdf": "PDF", "sms": "SMS", "vat": "VAT", "gst": "GST",
}

func Humanise(name string) string {
	words := strings.Split(name, "_")
	kept := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" {
			continue
		}
		lower := strings.ToLower(word)
		if acronym, ok := acronyms[lower]; ok {
			kept = append(kept, acronym)
			continue
		}
		if len(kept) == 0 {
			kept = append(kept, strings.ToUpper(lower[:1])+lower[1:])
			continue
		}
		kept = append(kept, lower)
	}
	return strings.Join(kept, " ")
}

func Titleise(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' && i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func Acronym(word string) (string, bool) {
	acronym, ok := acronyms[strings.ToLower(strings.TrimSpace(word))]
	return acronym, ok
}
