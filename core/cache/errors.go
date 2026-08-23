package cache

import "errors"

var (
	ErrUnsupported    = errors.New("coyote/cache: the backend does not support this operation")
	ErrUnreachable    = errors.New("coyote/cache: cache is unreachable")
	ErrClosed         = errors.New("coyote/cache: cache is closed")
	ErrBuildAbandoned = errors.New("coyote/cache: the build for this key did not finish")
)
