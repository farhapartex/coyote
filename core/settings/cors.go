package settings

import "time"

type CORS struct {
	Origins          []string
	Methods          []string
	Headers          []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func (c CORS) Enabled() bool { return len(c.Origins) > 0 }

func (c CORS) AllowsAnyOrigin() bool {
	for _, origin := range c.Origins {
		if origin == "*" {
			return true
		}
	}
	return false
}

func (c CORS) MethodList() []string {
	if len(c.Methods) > 0 {
		return c.Methods
	}
	return []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
}

func (c CORS) HeaderList() []string {
	if len(c.Headers) > 0 {
		return c.Headers
	}
	return []string{"Content-Type", "X-CSRF-Token", "X-Request-Id"}
}
