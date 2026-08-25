package mail

import "context"

type Sender interface {
	Send(ctx context.Context, message Message) error
}
