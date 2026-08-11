package auth

import "regexp"

var (
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._@+-]{3,64}$`)
	emailPattern    = regexp.MustCompile(`^[^@\s]+@[^@\s.]+\.[^@\s]+$`)
)
