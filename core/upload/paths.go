package upload

import (
	"path"
	"strings"
)

func CleanPath(prefix string) string {
	prefix = strings.TrimSpace(strings.ReplaceAll(prefix, `\`, "/"))
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return ""
	}

	kept := make([]string, 0, 4)
	for _, segment := range strings.Split(prefix, "/") {
		segment = strings.TrimSpace(segment)
		switch segment {
		case "", ".", "..":
			continue
		}
		if strings.ContainsRune(segment, 0) {
			continue
		}
		kept = append(kept, segment)
	}
	if len(kept) == 0 {
		return ""
	}
	return path.Join(kept...)
}

func join(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.Trim(part, "/"); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "/")
}
