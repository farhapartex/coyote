package cache

import (
	"context"
	"time"
)

type NoopStore struct{}

func NewNoopStore() *NoopStore { return &NoopStore{} }

func (*NoopStore) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }

func (*NoopStore) Set(context.Context, string, []byte, time.Duration) error { return nil }

func (*NoopStore) Delete(context.Context, string) error { return nil }

func (*NoopStore) Has(context.Context, string) (bool, error) { return false, nil }

func (*NoopStore) Clear(context.Context) error { return nil }

func (*NoopStore) ClearPrefix(context.Context, string) error { return nil }

func (*NoopStore) Ping(context.Context) error { return nil }

func (*NoopStore) Len() int { return 0 }
