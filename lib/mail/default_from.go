package mail

import (
	"context"
	"strings"
)

type defaultFromSender struct {
	inner Sender
	from  string
}

func WithDefaultFrom(inner Sender, from string) Sender {
	if inner == nil || strings.TrimSpace(from) == "" {
		return inner
	}
	return &defaultFromSender{inner: inner, from: from}
}

func (d *defaultFromSender) Send(ctx context.Context, message Message) error {
	if strings.TrimSpace(message.From) == "" {
		message.From = d.from
	}
	return d.inner.Send(ctx, message)
}
