package clientip

import (
	"net/http"
	"net/netip"
)

const ipv6BucketBits = 64

func Key(r *http.Request, trustedProxies int) string {
	return Bucket(From(r, trustedProxies))
}

func Bucket(address string) string {
	parsed, err := netip.ParseAddr(address)
	if err != nil {
		return address
	}
	parsed = parsed.WithZone("").Unmap()
	if parsed.Is4() {
		return parsed.String()
	}
	prefix, err := parsed.Prefix(ipv6BucketBits)
	if err != nil {
		return parsed.String()
	}
	return prefix.String()
}
