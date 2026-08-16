package upload

import (
	"path/filepath"
	"strings"
	"unicode"
)

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
}

func SafeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}

	name = strings.ReplaceAll(name, `\`, "/")
	name = filepath.Base(name)

	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r < 32 || r == 127:
			return -1
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			return '-'
		case unicode.IsSpace(r):
			return ' '
		}
		return r
	}, name)

	cleaned = strings.Trim(cleaned, " .")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "file"
	}
	if reservedNames[strings.ToLower(strings.TrimSuffix(cleaned, filepath.Ext(cleaned)))] {
		cleaned = "_" + cleaned
	}
	if len(cleaned) > 120 {
		extension := filepath.Ext(cleaned)
		cleaned = cleaned[:120-len(extension)] + extension
	}
	return cleaned
}
