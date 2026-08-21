package cache

import (
	"context"
	"time"
)

func GetOrSet(ctx context.Context, c Cache, key string, ttl time.Duration, build func(context.Context) ([]byte, error)) ([]byte, error) {
	if value, found, err := c.Get(ctx, key); err == nil && found {
		return value, nil
	}

	populate := func() ([]byte, error) {
		if value, found, err := c.Get(ctx, key); err == nil && found {
			return value, nil
		}
		value, err := build(ctx)
		if err != nil {
			return nil, err
		}
		_ = c.Set(ctx, key, value, ttl)
		return value, nil
	}

	if handle, ok := c.(*Handle); ok {
		return handle.flight.do(key, populate)
	}
	return populate()
}

func Remember[T any](ctx context.Context, c Cache, key string, ttl time.Duration, build func(context.Context) (T, error)) (T, error) {
	var zero T
	codec := codecOf(c)

	raw, err := GetOrSet(ctx, c, key, ttl, func(ctx context.Context) ([]byte, error) {
		value, err := build(ctx)
		if err != nil {
			return nil, err
		}
		return codec.Encode(value)
	})
	if err != nil {
		return zero, err
	}

	var value T
	if err := codec.Decode(raw, &value); err != nil {
		rebuilt, buildErr := build(ctx)
		if buildErr != nil {
			return zero, err
		}
		_ = c.Delete(ctx, key)
		return rebuilt, nil
	}
	return value, nil
}

func GetJSON[T any](ctx context.Context, c Cache, key string) (T, bool, error) {
	var value T
	raw, found, err := c.Get(ctx, key)
	if err != nil || !found {
		return value, false, err
	}
	if err := codecOf(c).Decode(raw, &value); err != nil {
		return value, false, err
	}
	return value, true, nil
}

func SetJSON[T any](ctx context.Context, c Cache, key string, value T, ttl time.Duration) error {
	raw, err := codecOf(c).Encode(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, raw, ttl)
}

func codecOf(c Cache) Codec {
	if handle, ok := c.(*Handle); ok && handle.codec != nil {
		return handle.codec
	}
	return JSONCodec{}
}
