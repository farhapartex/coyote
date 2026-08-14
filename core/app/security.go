package app

import "github.com/farhapartex/coyote/core/middleware"

func requestID(s Settings) Middleware {
	if s.Security.TrustRequestID {
		return middleware.TrustedRequestID
	}
	return middleware.RequestID
}
