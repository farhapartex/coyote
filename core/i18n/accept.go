package i18n

import (
	"sort"
	"strconv"
	"strings"
)

const maxAcceptEntries = 32

type preference struct {
	tag     string
	quality float64
	order   int
}

func ParseAcceptLanguage(header string) []string {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}

	entries := []preference{}
	for index, part := range strings.Split(header, ",") {
		if index >= maxAcceptEntries {
			break
		}
		entry, ok := parseAcceptEntry(part, index)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].quality != entries[j].quality {
			return entries[i].quality > entries[j].quality
		}
		return entries[i].order < entries[j].order
	})

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.tag)
	}
	return out
}

func parseAcceptEntry(part string, order int) (preference, bool) {
	tag, parameters, _ := strings.Cut(strings.TrimSpace(part), ";")
	tag = strings.TrimSpace(tag)
	if tag == "" || tag == "*" {
		return preference{}, false
	}
	if !plausibleTag(tag) {
		return preference{}, false
	}

	quality := 1.0
	for _, parameter := range strings.Split(parameters, ";") {
		name, value, found := strings.Cut(parameter, "=")
		if !found || strings.TrimSpace(name) != "q" {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || parsed < 0 || parsed > 1 {
			return preference{}, false
		}
		quality = parsed
	}
	if quality == 0 {
		return preference{}, false
	}
	return preference{tag: Normalise(tag), quality: quality, order: order}, true
}

func plausibleTag(tag string) bool {
	if len(tag) > 35 {
		return false
	}
	for _, char := range tag {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9', char == '-', char == '_':
		default:
			return false
		}
	}
	return true
}

func Match(wanted []string, supported []string) string {
	if len(supported) == 0 {
		return ""
	}
	available := make(map[string]string, len(supported)*2)
	for _, tag := range supported {
		normalised := Normalise(tag)
		available[strings.ToLower(normalised)] = normalised
	}

	for _, candidate := range wanted {
		if match, found := available[strings.ToLower(candidate)]; found {
			return match
		}
	}
	for _, candidate := range wanted {
		base := BaseOf(candidate)
		if match, found := available[base]; found {
			return match
		}
		for _, tag := range supported {
			if BaseOf(tag) == base {
				return Normalise(tag)
			}
		}
	}
	return ""
}
