package cache

type Codec interface {
	Encode(value any) ([]byte, error)
	Decode(raw []byte, target any) error
}
