package i18n

import "strings"

var rightToLeft = map[string]bool{
	"ar": true, "arc": true, "dv": true, "fa": true, "ha": true, "he": true,
	"khw": true, "ks": true, "ku": true, "ps": true, "sd": true, "ug": true,
	"ur": true, "yi": true,
}

func DirectionOf(tag string) string {
	if rightToLeft[BaseOf(tag)] {
		return RightToLeft
	}
	return LeftToRight
}

func BaseOf(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, "_", "-")
	if base, _, found := strings.Cut(tag, "-"); found {
		return base
	}
	return tag
}

func Normalise(tag string) string {
	tag = strings.TrimSpace(strings.ReplaceAll(tag, "_", "-"))
	if tag == "" {
		return ""
	}

	parts := strings.Split(tag, "-")
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		switch len(parts[i]) {
		case 2:
			parts[i] = strings.ToUpper(parts[i])
		case 4:
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		default:
			parts[i] = strings.ToLower(parts[i])
		}
	}
	return strings.Join(parts, "-")
}
