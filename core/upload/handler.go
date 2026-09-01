package upload

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var inlineTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
}

type HandlerOptions struct {
	Prefix  string
	Private bool
	Secret  string
	MaxAge  time.Duration
}

func (s *Service) Handler(opts HandlerOptions) http.Handler {
	prefix := "/" + strings.Trim(opts.Prefix, "/") + "/"
	maxAge := opts.MaxAge

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, prefix)
		if key == "" || key == r.URL.Path {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(key, StagedPrefix+"/") || strings.HasPrefix(key, TrashPrefix+"/") {
			http.NotFound(w, r)
			return
		}
		if opts.Private && !ValidSignature(opts.Secret, key, r.URL.Query().Get("expires"), r.URL.Query().Get("signature"), maxAge) {
			http.Error(w, "403 forbidden", http.StatusForbidden)
			return
		}

		stat, err := s.store.Stat(r.Context(), key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		file, err := s.store.Open(r.Context(), key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		kind := ContentTypeFor(key)
		w.Header().Set("Content-Type", kind)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !inlineTypes[kind] {
			w.Header().Set("Content-Disposition", "attachment")
		}
		if opts.Private {
			w.Header().Set("Cache-Control", "private, no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		http.ServeContent(w, r, key, stat.Modified, file)
	})
}

func ContentTypeFor(key string) string {
	switch {
	case strings.HasSuffix(key, ".jpg"), strings.HasSuffix(key, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(key, ".png"):
		return "image/png"
	case strings.HasSuffix(key, ".gif"):
		return "image/gif"
	case strings.HasSuffix(key, ".webp"):
		return "image/webp"
	case strings.HasSuffix(key, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(key, ".txt"):
		return "text/plain; charset=utf-8"
	case strings.HasSuffix(key, ".csv"):
		return "text/csv"
	case strings.HasSuffix(key, ".zip"):
		return "application/zip"
	}
	return "application/octet-stream"
}

func Sign(secret, key string, expires time.Time) string {
	stamp := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d:%s:%d:%s", len(key), key, len(stamp), stamp)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ValidSignature(secret, key, expires, signature string, maxAge time.Duration) bool {
	if secret == "" || signature == "" || expires == "" {
		return false
	}
	seconds, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		return false
	}
	deadline := time.Unix(seconds, 0)
	now := time.Now()
	if now.After(deadline) {
		return false
	}
	if maxAge > 0 && deadline.Sub(now) > maxAge {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(Sign(secret, key, deadline)))
}

func (s *Service) SignedURL(prefix, secret string, ref Ref, lifetime time.Duration) string {
	expires := time.Now().Add(lifetime)
	return "/" + strings.Trim(prefix, "/") + "/" + string(ref) +
		"?expires=" + strconv.FormatInt(expires.Unix(), 10) +
		"&signature=" + Sign(secret, string(ref), expires)
}

func (s *Service) URL(prefix string, ref Ref) string {
	if ref.Empty() {
		return ""
	}
	return "/" + strings.Trim(prefix, "/") + "/" + string(ref)
}
