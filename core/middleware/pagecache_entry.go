package middleware

import (
	"bytes"
	"encoding/gob"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var hopByHop = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailer", "Transfer-Encoding", "Upgrade", "Set-Cookie",
}

var perRequest = []string{
	CacheStatusHeader, "Content-Security-Policy", "Content-Security-Policy-Report-Only",
	"X-Request-Id", "RateLimit-Limit", "RateLimit-Remaining", "Retry-After",
}

type pageEntry struct {
	Status int
	Header http.Header
	Body   []byte
	Vary   []string
}

func (e pageEntry) encode() ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := gob.NewEncoder(buffer).Encode(e); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func decodePageEntry(raw []byte) (pageEntry, error) {
	var entry pageEntry
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&entry); err != nil {
		return pageEntry{}, err
	}
	return entry, nil
}

func (e pageEntry) writeTo(w http.ResponseWriter) {
	target := w.Header()
	for name, values := range e.Header {
		for index, value := range values {
			if index == 0 {
				target.Set(name, value)
				continue
			}
			target.Add(name, value)
		}
	}
	target.Set(CacheStatusHeader, CacheHit)
	w.WriteHeader(e.Status)
	_, _ = w.Write(e.Body)
}

type pageRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	cookie bool
}

func (p *pageRecorder) WriteHeader(status int) {
	if p.status == 0 {
		p.status = status
		if len(p.Header().Values("Set-Cookie")) > 0 {
			p.cookie = true
		}
	}
	p.ResponseWriter.WriteHeader(status)
}

func (p *pageRecorder) Write(body []byte) (int, error) {
	if p.status == 0 {
		p.WriteHeader(http.StatusOK)
	}
	p.body.Write(body)
	return p.ResponseWriter.Write(body)
}

func (p *pageRecorder) Flush() {
	if flusher, ok := p.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (p *pageRecorder) Unwrap() http.ResponseWriter { return p.ResponseWriter }

func (p *pageRecorder) entry() pageEntry {
	header := http.Header{}
	for name, values := range p.Header() {
		if slicesContainsFold(hopByHop, name) || slicesContainsFold(perRequest, name) {
			continue
		}
		header[name] = append([]string(nil), values...)
	}
	return pageEntry{
		Status: p.status,
		Header: header,
		Body:   bytes.Clone(p.body.Bytes()),
		Vary:   keyedVaryNames(p.Header().Values("Vary")),
	}
}

func keyedVaryNames(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, name := range strings.Split(value, ",") {
			canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
			if canonical == "" || canonical == "Cookie" || seen[canonical] {
				continue
			}
			seen[canonical] = true
			out = append(out, canonical)
		}
	}
	return out
}

func slicesContainsFold(list []string, candidate string) bool {
	for _, entry := range list {
		if strings.EqualFold(entry, candidate) {
			return true
		}
	}
	return false
}

func maxAgeOf(header http.Header) time.Duration {
	directives := strings.Split(header.Get("Cache-Control"), ",")
	for _, name := range []string{"s-maxage", "max-age"} {
		for _, directive := range directives {
			key, value, found := strings.Cut(strings.TrimSpace(directive), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), name) {
				continue
			}
			seconds, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	return 0
}

func hasDirective(header http.Header, names ...string) bool {
	directives := strings.Split(header.Get("Cache-Control"), ",")
	for _, directive := range directives {
		key, _, _ := strings.Cut(strings.TrimSpace(directive), "=")
		if slicesContainsFold(names, strings.TrimSpace(key)) {
			return true
		}
	}
	return false
}
