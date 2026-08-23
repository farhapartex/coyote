package i18n

type Entry struct {
	Context  string
	Singular string
	Plural   string
	Forms    []string
	Fuzzy    bool
	Header   bool
}

func (e Entry) Key() string { return messageKey(e.Context, e.Singular) }

func (e Entry) Translated() bool {
	if e.Fuzzy {
		return false
	}
	for _, form := range e.Forms {
		if form != "" {
			return true
		}
	}
	return false
}

type Iterable interface {
	Catalog
	Entries() []Entry
}

func Entries(catalog Catalog) []Entry {
	if listable, ok := catalog.(Iterable); ok {
		return listable.Entries()
	}
	return nil
}
