package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

func AllowedHosts(hosts []string, debug bool) Middleware {
	allowAny := len(hosts) == 0 && debug
	patterns := make([]string, 0, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "*" {
			allowAny = true
			break
		}
		if host != "" {
			patterns = append(patterns, host)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowAny || hostAllowed(r.Host, patterns) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "400 bad request: host not allowed", http.StatusBadRequest)
		})
	}
}

func hostAllowed(requestHost string, patterns []string) bool {
	host := strings.ToLower(requestHost)
	if h, port, err := net.SplitHostPort(host); err == nil {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return false
		}
		host = h
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	host = strings.TrimSuffix(host, ".")
	for _, pattern := range patterns {
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, ".") && (strings.HasSuffix(host, pattern) || host == pattern[1:]) {
			return true
		}
	}
	return false
}
