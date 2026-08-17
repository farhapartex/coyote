package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound = errors.New("coyote/storage: no such object")
	ErrBadKey   = errors.New("coyote/storage: invalid key")
)

type Stat struct {
	Key      string
	Size     int64
	Modified time.Time
}

type Storage interface {
	Save(ctx context.Context, key string, r io.Reader) (Stat, error)
	Open(ctx context.Context, key string) (io.ReadSeekCloser, error)
	Stat(ctx context.Context, key string) (Stat, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool
	Move(ctx context.Context, from, to string) error
	List(ctx context.Context, prefix string) ([]Stat, error)
}
