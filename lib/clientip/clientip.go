package clientip

import (
	"net"
	"net/http"
	"strings"
)

const forwardedHeader = "X-Forwarded-For"

func From(r *http.Request, trustedProxies int) string {
	if r == nil {
		return ""
	}
	if trustedProxies > 0 {
		if client := forwarded(r, trustedProxies); client != "" {
			return client
		}
	}
	return remote(r)
}

func forwarded(r *http.Request, trustedProxies int) string {
	hops := hops(r)
	index := len(hops) - trustedProxies
	if index < 0 {
		return ""
	}
	if net.ParseIP(hops[index]) == nil {
		return ""
	}
	return hops[index]
}

func hops(r *http.Request) []string {
	out := []string{}
	for _, value := range r.Header.Values(forwardedHeader) {
		for _, hop := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(hop); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func remote(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
