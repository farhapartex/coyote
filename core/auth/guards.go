package auth

import (
	"net/http"
	"net/url"
)

func (s *Service) RequireLogin(loginURL string) func(http.Handler) http.Handler {
	return s.guard(loginURL, func(u *User) bool { return u != nil })
}

func (s *Service) RequireSuperadmin(loginURL string) func(http.Handler) http.Handler {
	return s.guard(loginURL, func(u *User) bool { return u != nil && u.IsSuperadmin })
}

func (s *Service) guard(loginURL string, allow func(*User) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := s.CurrentUser(r)
			if allow(u) {
				next.ServeHTTP(w, r)
				return
			}
			if u != nil {
				http.Error(w, "403 forbidden", http.StatusForbidden)
				return
			}
			target := loginURL
			if target == "" {
				http.Error(w, "401 unauthorized", http.StatusUnauthorized)
				return
			}
			redirect := url.URL{Path: target}
			query := redirect.Query()
			query.Set("next", r.URL.RequestURI())
			redirect.RawQuery = query.Encode()
			http.Redirect(w, r, redirect.String(), http.StatusSeeOther)
		})
	}
}
