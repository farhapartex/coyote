package cache

import "context"

func Close(c Cache) error {
	if closer, ok := c.(Closer); ok {
		return closer.Close()
	}
	return nil
}

func Ping(ctx context.Context, c Cache) error {
	if pinger, ok := c.(Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func Length(c Cache) int {
	if measured, ok := c.(Measured); ok {
		return measured.Len()
	}
	return UnknownLength
}

func Evictions(c Cache) int64 {
	if evicter, ok := c.(Evicter); ok {
		return evicter.Evictions()
	}
	return 0
}
