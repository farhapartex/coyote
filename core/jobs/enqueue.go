package jobs

import (
	"context"
	"time"
)

func Enqueue[T any](ctx context.Context, target Queue, kind string, args T, opts ...Options) (string, error) {
	if target == nil {
		return "", ErrNoQueue
	}
	payload, err := encodePayload(args)
	if err != nil {
		return "", err
	}
	job := firstOption(opts).job(kind, payload, time.Now())
	if err := job.Valid(); err != nil {
		return "", err
	}
	return target.Enqueue(ctx, job)
}
