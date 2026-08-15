package session

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var ErrNoSession = errors.New("coyote/session: no session in request context")

type Options struct {
	Store      Store
	Sealer     *Sealer
	CookieName string
	Lifetime   time.Duration
	Rolling    bool
	Secure     bool
	HTTPOnly   bool
	SameSite   http.SameSite
	Path       string
	Domain     string
	OnError    func(error)
}

type Manager struct {
	store      Store
	carrier    carrier
	cookieName string
	lifetime   time.Duration
	rolling    bool
	secure     bool
	httpOnly   bool
	sameSite   http.SameSite
	path       string
	domain     string
	onError    func(error)
}

func NewManager(opts Options) *Manager {
	if opts.Sealer == nil && opts.Store == nil {
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
		carrier:    carrierFor(opts),
		cookieName: opts.CookieName,
		lifetime:   opts.Lifetime,
		rolling:    opts.Rolling,
		secure:     opts.Secure,
		httpOnly:   opts.HTTPOnly,
		sameSite:   opts.SameSite,
		path:       opts.Path,
		domain:     opts.Domain,
		onError:    opts.OnError,
	}
}

func carrierFor(opts Options) carrier {
	if opts.Sealer != nil {
		return cookieCarrier{sealer: opts.Sealer}
	}
	return storeCarrier{store: opts.Store}
}

func (m *Manager) Store() Store { return m.store }

func (m *Manager) Stateless() bool { return m.store == nil }

func (m *Manager) report(err error) {
	if m.onError != nil {
		m.onError(err)
	}
}

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
		if sess, ok := m.carrier.load(cookie.Value); ok {
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
