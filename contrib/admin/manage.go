package admin

import (
	"errors"
	"fmt"
	"github.com/farhapartex/coyote/core/auth"
)

var (
	ErrReservedSlug  = errors.New("coyote/admin: that path is reserved by the portal")
	ErrDuplicateSlug = errors.New("coyote/admin: a resource is already registered on that path")
	ErrNotMountedYet = errors.New("coyote/admin: Manage must be called after Mount")
)

var reservedSlugs = map[string]bool{
	"login":    true,
	"logout":   true,
	"users":    true,
	"roles":    true,
	"sessions": true,
	"jobs":     true,
	"new":      true,
	"s":        true,
}

func (a *Admin) Manage(resources ...Resource) error {
	if a.guarded == nil {
		return ErrNotMountedYet
	}
	for _, resource := range resources {
		schema, err := a.app.Describe(resource.Entity())
		if err != nil {
			return err
		}
		entry := describeResource(schema, resource)

		slug := entry.Slug()
		if reservedSlugs[slug] {
			return fmt.Errorf("%w: %q", ErrReservedSlug, slug)
		}
		if _, exists := a.resources.bySlug(slug); exists == nil {
			return fmt.Errorf("%w: %q", ErrDuplicateSlug, slug)
		}

		a.resources.add(entry)
		a.mountResource(entry)
	}
	return nil
}

func (a *Admin) mountResource(entry managed) {
	base := "/" + entry.Slug()
	a.guarded.Get(base, a.permit(entry, auth.ActionRead, a.resourceList(entry)))
	a.guarded.Get(base+"/new", a.permit(entry, auth.ActionCreate, a.resourceForm(entry)))
	a.guarded.Post(base+"/new", a.permit(entry, auth.ActionCreate, a.resourceCreate(entry)))
	a.guarded.Get(base+"/{id}", a.permit(entry, auth.ActionRead, a.resourceForm(entry)))
	a.guarded.Post(base+"/{id}", a.permit(entry, auth.ActionUpdate, a.resourceUpdate(entry)))
	a.guarded.Post(base+"/{id}/delete", a.permit(entry, auth.ActionDelete, a.resourceDelete(entry)))
	a.guarded.Post(base+"/bulk", a.permit(entry, auth.ActionRead, a.resourceBulk(entry)))
}

func (a *Admin) MustManage(resources ...Resource) {
	if err := a.Manage(resources...); err != nil {
		panic(fmt.Errorf("coyote/admin: %w", err))
	}
}

func (a *Admin) Managed() []string {
	entries := a.resources.all()
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Slug())
	}
	return out
}
