package settings

import (
	"strconv"
	"strings"
)

func (s Settings) validateAuth(add func(string)) {
	if s.Auth.PasswordMinLength < 6 {
		add("Auth.PasswordMinLength must be at least 6")
	}
	if s.Auth.PBKDF2Iterations < 1000 {
		add("Auth.PBKDF2Iterations must be at least 1000")
	}
	if s.Auth.LoginURL != "" && !strings.HasPrefix(s.Auth.LoginURL, "/") {
		add("Auth.LoginURL must start with \"/\"")
	}
}

func (s Settings) validateTemplates(add func(string)) {
	if s.Templates.Layout == "" {
		add("Templates.Layout is empty")
	}
	if s.Templates.FS != nil && s.Templates.Dir != "" {
		add("set either Templates.FS or Templates.Dir, not both")
	}
}

func (s Settings) validateStatic(add func(string)) {
	if !strings.HasPrefix(s.Static.URL, "/") || !strings.HasSuffix(s.Static.URL, "/") {
		add("Static.URL must start and end with \"/\"")
	}
	if s.Static.FS != nil && s.Static.Dir != "" {
		add("set either Static.FS or Static.Dir, not both")
	}
}

func (s Settings) validateAdmin(add func(string)) {
	if !strings.HasPrefix(s.Admin.Prefix, "/") {
		add("Admin.Prefix must start with \"/\"")
	}
	if s.Admin.Prefix == "/" {
		add("Admin.Prefix cannot be \"/\", it would take over every route")
	}
	if strings.HasSuffix(s.Admin.Prefix, "/") && s.Admin.Prefix != "/" {
		add("Admin.Prefix must not end with \"/\"")
	}
	if s.Admin.SiteName == "" {
		add("Admin.SiteName is empty")
	}
}

func (s Settings) validateSecurity(add func(string)) {
	if s.Security.CSPReportOnly && s.Security.CSP == "" {
		add("Security.CSPReportOnly is set but Security.CSP is empty, so no policy would be reported")
	}

	cors := s.Security.CORS
	if cors.AllowsAnyOrigin() && cors.AllowCredentials {
		add("Security.CORS cannot combine the \"*\" origin with AllowCredentials; " +
			"browsers reject that pairing, so list the origins you mean")
	}
	if cors.AllowsAnyOrigin() && len(cors.Origins) > 1 {
		add("Security.CORS lists \"*\" alongside other origins; remove the others or drop the wildcard")
	}
	for _, origin := range cors.Origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			add("Security.CORS contains an empty origin")
			continue
		}
		if trimmed == "*" {
			continue
		}
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			add("Security.CORS origin " + strconv.Quote(trimmed) + " needs a scheme, for example https://" + trimmed)
		}
		if strings.HasSuffix(trimmed, "/") {
			add("Security.CORS origin " + strconv.Quote(trimmed) + " must not end with a slash")
		}
	}
	if s.Security.CompressLevel != 0 && (s.Security.CompressLevel < 1 || s.Security.CompressLevel > 9) {
		add("Security.CompressLevel must be between 1 and 9, or 0 for the default")
	}
	if s.Security.CompressLevel != 0 && !s.Security.Compress {
		add("Security.CompressLevel is set but Security.Compress is false")
	}

	if cors.MaxAge < 0 {
		add("Security.CORS.MaxAge cannot be negative")
	}
}

func (s Settings) validateLogging(add func(string)) {
	switch strings.ToLower(s.Logging.Level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		add("Logging.Level must be one of \"debug\", \"info\", \"warn\", \"error\"")
	}
	switch strings.ToLower(s.Logging.Format) {
	case "text", "json":
	default:
		add("Logging.Format must be \"text\" or \"json\"")
	}
}
