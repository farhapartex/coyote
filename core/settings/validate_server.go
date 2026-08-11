package settings

import "strings"

func (s Settings) validateServer(add func(string)) {
	if s.Server.Port < 1 || s.Server.Port > 65535 {
		add("Server.Port must be between 1 and 65535")
	}
	if s.Server.ShutdownTimeout < 0 {
		add("Server.ShutdownTimeout cannot be negative")
	}
}

func (s Settings) validateSessions(add func(string)) {
	if s.Sessions.CookieName == "" {
		add("Sessions.CookieName is empty")
	}
	if strings.ContainsAny(s.Sessions.CookieName, " ;,\t\n") {
		add("Sessions.CookieName contains characters that are not valid in a cookie name")
	}
	if s.Sessions.Lifetime <= 0 {
		add("Sessions.Lifetime must be greater than zero")
	}
	switch s.Sessions.SameSite {
	case SameSiteLax, SameSiteStrict:
	case SameSiteNone:
		if !s.Sessions.Secure {
			add("Sessions.SameSite is \"none\", which browsers only accept when Sessions.Secure is true")
		}
	default:
		add("Sessions.SameSite must be \"lax\", \"strict\" or \"none\"")
	}
	if !strings.HasPrefix(s.Sessions.Path, "/") {
		add("Sessions.Path must start with \"/\"")
	}
}
