package app

import (
	"github.com/farhapartex/coyote/core/auth"
)

var internalResources = map[string]bool{
	"sessions":         true,
	"role_permissions": true,
	"user_roles":       true,
}

func (a *App) PermissionResources() ([]string, error) {
	out := []string{}
	for _, m := range a.Models() {
		schema, err := a.Describe(m.Entity)
		if err != nil {
			return nil, err
		}
		if internalResources[schema.Table] {
			continue
		}
		out = append(out, schema.Table)
	}
	return out, nil
}

func (a *App) SyncPermissions() (auth.SyncReport, error) {
	permissions := a.Auth.Permissions()
	if permissions == nil {
		return auth.SyncReport{}, nil
	}
	resources, err := a.PermissionResources()
	if err != nil {
		return auth.SyncReport{}, err
	}
	return permissions.SyncPermissions(resources)
}
