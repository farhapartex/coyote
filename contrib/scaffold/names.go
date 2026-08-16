package scaffold

import "strings"

func ValidName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "_") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func ValidModule(path string) bool {
	if path == "" || len(path) > 255 {
		return false
	}
	if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, r := range segment {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			case r == '-', r == '_', r == '.', r == '~':
			default:
				return false
			}
		}
	}
	return true
}

func Titleise(name string) string {
	fields := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, field := range fields {
		fields[i] = strings.ToUpper(field[:1]) + field[1:]
	}
	if len(fields) == 0 {
		return name
	}
	return strings.Join(fields, " ")
}
