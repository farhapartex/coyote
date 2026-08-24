package i18n

type Loader interface {
	Load(tag string) (Catalog, error)
}
