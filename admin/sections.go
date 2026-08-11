package admin

import (
	"net/http"
	"sort"
	"strings"
)

type Section struct {
	Name        string
	Slug        string
	Description string
	Handler     http.Handler
}

func (a *Admin) Register(s Section) {
	if s.Slug == "" {
		s.Slug = strings.ToLower(strings.ReplaceAll(s.Name, " ", "-"))
	}
	a.sections = append(a.sections, s)
	sort.Slice(a.sections, func(i, j int) bool { return a.sections[i].Name < a.sections[j].Name })
	if s.Handler != nil {
		guarded := a.router.Group("", a.app.Auth.RequireSuperadmin(a.prefix+"/login"))
		guarded.Mount("/s/"+s.Slug, s.Handler)
	}
}
