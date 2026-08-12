package model

import "strings"

var acronyms = map[string]string{
	"id": "ID", "sku": "SKU", "url": "URL", "uri": "URI", "api": "API",
	"ip": "IP", "uuid": "UUID", "html": "HTML", "css": "CSS", "json": "JSON",
	"xml": "XML", "pdf": "PDF", "sms": "SMS", "vat": "VAT", "gst": "GST",
}

var sensitiveNames = map[string]bool{
	"password": true,
	"secret":   true,
	"token":    true,
	"api_key":  true,
}

func Humanise(column string) string {
	words := strings.Split(column, "_")
	kept := make([]string, 0, len(words))
	for i, w := range words {
		if w == "" {
			continue
		}
		lower := strings.ToLower(w)
		if i == len(words)-1 && lower == "at" && len(words) > 1 {
			continue
		}
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

func isSensitive(column string) bool {
	if sensitiveNames[column] {
		return true
	}
	return strings.HasSuffix(column, "_password") || strings.HasSuffix(column, "_secret")
}
