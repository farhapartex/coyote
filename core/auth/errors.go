package auth

import "errors"

var (
	ErrUserNotFound   = errors.New("coyote/auth: user not found")
	ErrUserExists     = errors.New("coyote/auth: username already taken")
	ErrEmailExists    = errors.New("coyote/auth: email already registered")
	ErrInvalidUser    = errors.New("coyote/auth: invalid username")
	ErrInvalidEmail   = errors.New("coyote/auth: invalid email address")
	ErrLastSuperadmin = errors.New("coyote/auth: cannot remove the last superadmin")
)
