package session

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"
)

type contextKey struct{}

var sessionContextKey contextKey

var ErrNoSession = errors.New("coyote/session: no session in request context")

type Options struct {
	Store      Store
	CookieName string
	Lifetime   time.Duration
	Rolling    bool
	Secure     bool
	HTTPOnly   bool
	SameSite   http.SameSite
	Path       string
	Domain     string
}

type Manager struct {
	store      Store
	cookieName string
	lifetime   time.Duration
	rolling    bool
	secure     bool
	httpOnly   bool
	sameSite   http.SameSite
	path       string
	domain     string
}

func NewManager(opts Options) *Manager {
	if opts.Store == nil {
		opts.Store = NewMemoryStore(5 * time.Minute)
	}
	if opts.CookieName == "" {
		opts.CookieName = "coyote_session"
	}
	if opts.Lifetime <= 0 {
		opts.Lifetime = 12 * time.Hour
	}
	if opts.SameSite == 0 {
		opts.SameSite = http.SameSiteLaxMode
	}
	if opts.Path == "" {
		opts.Path = "/"
	}
	return &Manager{
		store:      opts.Store,
		cookieName: opts.CookieName,
		lifetime:   opts.Lifetime,
		rolling:    opts.Rolling,
		secure:     opts.Secure,
		httpOnly:   opts.HTTPOnly,
		sameSite:   opts.SameSite,
		path:       opts.Path,
		domain:     opts.Domain,
	}
}

func (m *Manager) Store() Store { return m.store }

func (m *Manager) CookieName() string { return m.cookieName }

func (m *Manager) Lifetime() time.Duration { return m.lifetime }

func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := m.load(r)
		sw := &sessionWriter{ResponseWriter: w, manager: m, session: sess}
		r = r.WithContext(context.WithValue(r.Context(), sessionContextKey, sess))
		defer sw.commit()
		next.ServeHTTP(sw, r)
	})
}

func (m *Manager) load(r *http.Request) *Session {
	cookie, err := r.Cookie(m.cookieName)
	if err == nil && cookie.Value != "" {
		if sess, ok := m.store.Load(cookie.Value); ok {
			if m.rolling {
				sess.touch(m.lifetime)
			}
			return sess
		}
	}
	return m.blank()
}

func (m *Manager) blank() *Session {
	id, err := newID()
	if err != nil {
		id = "invalid"
	}
	return newSession(id, m.lifetime)
}

func From(ctx context.Context) *Session {
	sess, _ := ctx.Value(sessionContextKey).(*Session)
	return sess
}

func FromRequest(r *http.Request) *Session {
	return From(r.Context())
}

func (m *Manager) Renew(r *http.Request) error {
	sess := FromRequest(r)
	if sess == nil {
		return ErrNoSession
	}
	id, err := newID()
	if err != nil {
		return err
	}
	sess.renew(id)
	return nil
}

func (m *Manager) Destroy(r *http.Request) error {
	sess := FromRequest(r)
	if sess == nil {
		return ErrNoSession
	}
	sess.Destroy()
	return nil
}

func (m *Manager) CSRFToken(r *http.Request) string {
	sess := FromRequest(r)
	if sess == nil {
		return ""
	}
	if token := sess.GetString(csrfKey); token != "" {
		return token
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	sess.Set(csrfKey, token)
	return token
}

func (m *Manager) ValidCSRF(r *http.Request, candidate string) bool {
	sess := FromRequest(r)
	if sess == nil || candidate == "" {
		return false
	}
	token := sess.GetString(csrfKey)
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1
}

type sessionWriter struct {
	http.ResponseWriter
	manager     *Manager
	session     *Session
	once        sync.Once
	wroteHeader bool
}

func (w *sessionWriter) WriteHeader(code int) {
	w.commit()
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *sessionWriter) Write(b []byte) (int, error) {
	w.commit()
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *sessionWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		w.commit()
		f.Flush()
	}
}

func (w *sessionWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *sessionWriter) commit() {
	w.once.Do(func() {
		if w.wroteHeader {
			return
		}
		m, sess := w.manager, w.session
		if old := sess.takeOldID(); old != "" {
			_ = m.store.Delete(old)
		}
		if sess.Destroyed() {
			_ = m.store.Delete(sess.ID())
			http.SetCookie(w.ResponseWriter, m.cookie("", -1))
			return
		}
		if !sess.Modified() {
			return
		}
		if err := m.store.Save(sess); err != nil {
			return
		}
		maxAge := int(time.Until(sess.ExpiresAt()).Seconds())
		if maxAge < 1 {
			maxAge = 1
		}
		http.SetCookie(w.ResponseWriter, m.cookie(sess.ID(), maxAge))
	})
}

func (m *Manager) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     m.cookieName,
		Value:    value,
		Path:     m.path,
		Domain:   m.domain,
		MaxAge:   maxAge,
		Secure:   m.secure,
		HttpOnly: m.httpOnly,
		SameSite: m.sameSite,
	}
}
