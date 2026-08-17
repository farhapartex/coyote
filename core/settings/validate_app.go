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
	if s.Auth.Throttle.Enabled {
		if s.Auth.Throttle.MaxAttempts < 1 {
			add("Auth.Throttle.MaxAttempts must be at least 1 when throttling is enabled")
		}
		if s.Auth.Throttle.Window <= 0 {
			add("Auth.Throttle.Window must be greater than zero when throttling is enabled")
		}
		if s.Auth.Throttle.Lockout < 0 {
			add("Auth.Throttle.Lockout cannot be negative")
		}
	}
}

func (s Settings) validatePagination(add func(string)) {
	if s.Pagination.PerPage < 0 {
		add("Pagination.PerPage cannot be negative; use 0 to turn pagination off")
	}
}

func (s Settings) validateUploads(add func(string)) {
	if !s.Uploads.Enabled {
		return
	}
	if s.Uploads.MaxSize <= 0 {
		add("Uploads.MaxSize must be greater than zero")
	}
	if len(s.Uploads.Allowed) == 0 {
		add("Uploads.Allowed is empty; list the content types you accept")
	}
	if s.Uploads.Dir == "" && s.Uploads.Storage == nil {
		add("Uploads.Dir is empty; set a directory or supply Uploads.Storage")
	}
	if s.Uploads.Serve {
		if s.Uploads.URL == "" || !strings.HasPrefix(s.Uploads.URL, "/") {
			add("Uploads.URL must start with \"/\" when Uploads.Serve is on")
		}
		if s.Uploads.Private && s.SecretKey == "" {
			add("Uploads.Private needs a SecretKey to sign URLs with")
		}
	}
	if s.Uploads.StageTTL < 0 || s.Uploads.TrashTTL < 0 {
		add("Uploads.StageTTL and Uploads.TrashTTL cannot be negative")
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

	limit := s.Security.RateLimit
	if limit.Requests < 0 {
		add("Security.RateLimit.Requests cannot be negative")
	}
	if limit.Window < 0 {
		add("Security.RateLimit.Window cannot be negative")
	}
	if limit.Requests > 0 && limit.Window == 0 {
		add("Security.RateLimit.Requests is set without a Window, so there is no period to limit over")
	}
	if limit.Window > 0 && limit.Requests == 0 {
		add("Security.RateLimit.Window is set without Requests, so nothing would be limited")
	}
	if limit.Burst < 0 {
		add("Security.RateLimit.Burst cannot be negative")
	}
	if limit.Burst > 0 && limit.Burst < limit.Requests {
		add("Security.RateLimit.Burst is smaller than Requests, which would throttle below the rate you asked for")
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
