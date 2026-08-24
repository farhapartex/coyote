package i18n

var endonyms = map[string]string{
	"ar": "العربية",
	"bn": "বাংলা",
	"de": "Deutsch",
	"en": "English",
	"es": "Español",
	"fa": "فارسی",
	"fr": "Français",
	"he": "עברית",
	"hi": "हिन्दी",
	"id": "Indonesia",
	"it": "Italiano",
	"ja": "日本語",
	"ko": "한국어",
	"nl": "Nederlands",
	"pl": "Polski",
	"pt": "Português",
	"ru": "Русский",
	"tr": "Türkçe",
	"uk": "Українська",
	"ur": "اردو",
	"vi": "Tiếng Việt",
	"zh": "中文",
}

func NameOf(tag string) string {
	if name, found := endonyms[Normalise(tag)]; found {
		return name
	}
	if name, found := endonyms[BaseOf(tag)]; found {
		return name
	}
	return Normalise(tag)
}
