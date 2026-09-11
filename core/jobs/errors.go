package jobs

import "errors"

var (
	ErrDuplicateKind        = errors.New("coyote/jobs: a handler is already registered for that kind")
	ErrUnknownKind          = errors.New("coyote/jobs: no handler is registered for that kind")
	ErrKindMissing          = errors.New("coyote/jobs: a job needs a kind")
	ErrPayloadInvalid       = errors.New("coyote/jobs: the payload does not decode into the handler's argument")
	ErrNoQueue              = errors.New("coyote/jobs: no queue is configured")
	ErrNotFound             = errors.New("coyote/jobs: no such job")
	ErrDuplicateFingerprint = errors.New("coyote/jobs: a job with that fingerprint is already queued")
	ErrPanicked             = errors.New("coyote/jobs: the handler panicked")
)
