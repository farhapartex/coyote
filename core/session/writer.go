package session

import (
	"net/http"
	"sync"
	"time"
)

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
