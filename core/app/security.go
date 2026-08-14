package app

import "github.com/farhapartex/coyote/core/middleware"

func securityPolicies(s Settings) []Middleware {
	var out []Middleware
	if s.Server.TLS.HSTS > 0 {
		out = append(out, middleware.HSTS(s.Server.TLS.HSTS))
	}
	if s.Security.CSP != "" {
		out = append(out, middleware.CSP(s.Security.CSP, s.Security.CSPReportOnly))
	}
	return out
}

func requestID(s Settings) Middleware {
	if s.Security.TrustRequestID {
		return middleware.TrustedRequestID
	}
	return middleware.RequestID
}
