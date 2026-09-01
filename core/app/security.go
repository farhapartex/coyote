package app

import (
	"github.com/farhapartex/coyote/core/middleware"
	"github.com/farhapartex/coyote/core/session"
)

func securityPolicies(s Settings) []Middleware {
	var out []Middleware
	if s.Server.TLS.HSTS > 0 {
		out = append(out, middleware.HSTS(s.Server.TLS.HSTS))
	}
	if s.Security.CSP != "" {
		out = append(out, middleware.CSP(s.Security.CSP, s.Security.CSPReportOnly))
	}
	if s.Security.CORS.Enabled() {
		out = append(out, middleware.CORS(s.Security.CORS))
	}
	if s.Security.Compress {
		out = append(out, middleware.Compress(s.Security.CompressLevel))
	}
	if s.Security.RateLimit.Enabled() {
		out = append(out, middleware.RateLimit(s.Security.RateLimit, s.Security.TrustedProxyCount))
	}
	return out
}

func csrfGuard(s Settings, sessions *session.Manager) Middleware {
	if !s.Security.CSRF {
		return nil
	}
	return middleware.CSRF(sessions, s.Security.CSRFExempt)
}

func requestID(s Settings) Middleware {
	if s.Security.TrustRequestID {
		return middleware.TrustedRequestID
	}
	return middleware.RequestID
}
