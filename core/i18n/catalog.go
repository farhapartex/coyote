package i18n

type Catalog interface {
	Tag() string
	Lookup(context, msgid string) (string, bool)
	LookupPlural(context, msgid string, n int) (string, bool)
	Plurals() int
	Header(name string) string
}
