package settings

import "strings"

func (s Settings) validateServer(add func(string)) {
	tls := s.Server.TLS
	if tls.Config == nil {
		if tls.CertFile != "" && tls.KeyFile == "" {
			add("Server.TLS.CertFile is set without Server.TLS.KeyFile")
		}
		if tls.KeyFile != "" && tls.CertFile == "" {
			add("Server.TLS.KeyFile is set without Server.TLS.CertFile")
		}
	}
	if tls.Autocert {
		if !tls.AcceptTOS {
			add("Server.TLS.Autocert needs Server.TLS.AcceptTOS set to true; " +
				"issuing a certificate means agreeing to the certificate authority's terms of service")
		}
		if tls.CertFile != "" || tls.KeyFile != "" {
			add("Server.TLS.Autocert cannot be combined with CertFile and KeyFile; choose one")
		}
		if tls.Config != nil {
			add("Server.TLS.Autocert cannot be combined with a supplied Config; put your own manager in the Config instead")
		}
		if tls.CacheDir == "" {
			add("Server.TLS.CacheDir is empty; certificates would be re-issued on every restart")
		}
		if len(s.AllowedHosts) == 0 {
			add("Server.TLS.Autocert needs AllowedHosts; it is the list of names certificates are issued for")
		}
		for _, host := range s.AllowedHosts {
			if strings.TrimSpace(host) == "*" {
				add("Server.TLS.Autocert cannot use the \"*\" host; a certificate authority needs real host names")
			}
		}
		if s.Server.Port != 443 {
			add("Server.TLS.Autocert validates over TLS on port 443, so Server.Port must be 443; " +
				"for anything else supply your own Server.TLS.Config")
		}
	}

	if tls.HSTS < 0 {
		add("Server.TLS.HSTS cannot be negative")
	}
	if tls.HSTS > 0 && !tls.Enabled() {
		add("Server.TLS.HSTS only makes sense when TLS is enabled; a browser that sees it will refuse plain HTTP for that long")
	}

	if s.Server.Port < 1 || s.Server.Port > 65535 {
		add("Server.Port must be between 1 and 65535")
	}
	if s.Server.ShutdownTimeout < 0 {
		add("Server.ShutdownTimeout cannot be negative")
	}
}

func (s Settings) validateSessions(add func(string)) {
	switch s.Sessions.Backend {
	case SessionsInMemory:
	case SessionsInDB:
		if s.Database().Engine == "" {
			add("Sessions.Backend is \"database\" but no database is configured")
		}
	case SessionsInCookie:
		if s.SecretKey == "" {
			add("Sessions.Backend is \"cookie\" but SecretKey is empty; the session is sealed with it")
		}
	default:
		add("Sessions.Backend must be \"memory\", \"database\" or \"cookie\"")
	}
	if s.Sessions.Backend != SessionsInMemory && s.Sessions.Store != nil {
		add("Sessions.Backend is \"" + string(s.Sessions.Backend) + "\" but Sessions.Store is also set; choose one")
	}

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
