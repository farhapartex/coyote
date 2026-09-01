package settings

type Security struct {
	TrustRequestID    bool
	TrustedProxyCount int
	FrameOptions      string
	PermissionsPolicy string
	CSRF              bool
	CSRFExempt        []string
	CSP               string
	CSPReportOnly     bool
	CORS              CORS
	Compress          bool
	CompressLevel     int
	RateLimit         RateLimit
}

const DefaultFrameOptions = "DENY"

const DefaultPermissionsPolicy = "camera=(), microphone=(), geolocation=()"

const DefaultCSP = "default-src 'self'; " +
	"script-src 'self' {nonce}; " +
	"style-src 'self' {nonce}; " +
	"img-src 'self' data:; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"frame-ancestors 'none'"
