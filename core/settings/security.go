package settings

type Security struct {
	TrustRequestID bool
	CSP            string
	CSPReportOnly  bool
	CORS           CORS
	Compress       bool
	CompressLevel  int
	RateLimit      RateLimit
}

const DefaultCSP = "default-src 'self'; " +
	"script-src 'self' {nonce}; " +
	"style-src 'self' {nonce}; " +
	"img-src 'self' data:; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"frame-ancestors 'none'"
