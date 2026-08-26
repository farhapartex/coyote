package mail

import "errors"

var (
	ErrNoSender        = errors.New("coyote/mail: the message has no From address")
	ErrNoRecipient     = errors.New("coyote/mail: the message has no recipients")
	ErrNoBody          = errors.New("coyote/mail: the message has neither a text nor an HTML body")
	ErrBadAddress      = errors.New("coyote/mail: unusable address")
	ErrHeaderInjection = errors.New("coyote/mail: a header value contains a line break")
	ErrNotConfigured   = errors.New("coyote/mail: no email backend is configured")
	ErrSendFailed      = errors.New("coyote/mail: the server refused the message")
	ErrTooLarge        = errors.New("coyote/mail: the message is larger than the limit")
)
