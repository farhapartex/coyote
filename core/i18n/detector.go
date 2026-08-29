package i18n

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

const cookieLifetime = 365 * 24 * time.Hour

type Detector struct {
	bundle *Bundle
	cookie string
	prefix bool
}

func NewDetector(bundle *Bundle, cookie string, urlPrefix bool) *Detector {
	if cookie == "" {
		cookie = DefaultCookieName
	}
	return &Detector{bundle: bundle, cookie: cookie, prefix: urlPrefix}
}

func (d *Detector) Bundle() *Bundle { return d.bundle }

func (d *Detector) CookieName() string { return d.cookie }

func (d *Detector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.bundle == nil {
			next.ServeHTTP(w, r)
			return
		}

		tag, stripped, canonical := d.fromPath(r.URL.Path)
		if canonical != "" {
			http.Redirect(w, r, d.canonicalTarget(r, canonical), http.StatusMovedPermanently)
			return
		}
		if tag == "" {
			tag = d.fromCookie(r)
		}
		if tag == "" {
			tag = d.fromHeader(r)
		}

		if d.bundle.Multilingual() {
			w.Header().Add("Vary", "Accept-Language")
		}

		locale := d.bundle.Locale(tag)
		request := r
		if stripped != "" && stripped != r.URL.Path {
			request = r.Clone(r.Context())
			request.URL.Path = stripped
		}
		next.ServeHTTP(w, request.WithContext(WithLocale(request.Context(), locale)))
	})
}

func (d *Detector) fromPath(path string) (string, string, string) {
	if !d.prefix {
		return "", "", ""
	}
	segment, rest := SplitPrefix(path)
	if segment == "" || !d.bundle.Supports(segment) {
		return "", "", ""
	}
	if Normalise(segment) == d.bundle.Default() {
		return "", "", rest
	}
	return Normalise(segment), rest, ""
}

func (d *Detector) canonicalTarget(r *http.Request, path string) string {
	target := url.URL{Path: path, RawQuery: r.URL.RawQuery}
	return target.String()
}

func (d *Detector) fromCookie(r *http.Request) string {
	cookie, err := r.Cookie(d.cookie)
	if err != nil || cookie.Value == "" {
		return ""
	}
	candidate := Normalise(cookie.Value)
	if !plausibleTag(cookie.Value) || !d.bundle.Supports(candidate) {
		return ""
	}
	return candidate
}

func (d *Detector) fromHeader(r *http.Request) string {
	header := r.Header.Get("Accept-Language")
	if header == "" {
		return ""
	}
	return Match(ParseAcceptLanguage(header), d.bundle.Supported())
}

func (d *Detector) Cookie(tag string) *http.Cookie {
	return &http.Cookie{
		Name:     d.cookie,
		Value:    Normalise(tag),
		Path:     "/",
		MaxAge:   int(cookieLifetime.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func (d *Detector) Choose(w http.ResponseWriter, tag string) bool {
	candidate := Normalise(tag)
	if d.bundle == nil || !d.bundle.Supports(candidate) {
		return false
	}
	http.SetCookie(w, d.Cookie(candidate))
	return true
}

func (d *Detector) SwitchHandler(fallback string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		chosen := strings.TrimSpace(r.Form.Get("locale"))
		d.Choose(w, chosen)

		target := safePath(r.Form.Get("next"), fallback)
		if locale := d.bundle.Locale(Normalise(chosen)); locale != nil && d.prefix {
			target = locale.Switch(target, Normalise(chosen))
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func safePath(next, fallback string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		if fallback == "" {
			return "/"
		}
		return fallback
	}
	return next
}
