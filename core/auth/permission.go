package auth

import (
	"sort"
	"strings"
	"time"
)

const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

func Actions() []string {
	return []string{ActionCreate, ActionRead, ActionUpdate, ActionDelete}
}

type Permission struct {
	ID        string `gorm:"primaryKey;size:64"`
	Resource  string `gorm:"uniqueIndex:idx_permission_codename;size:100;not null"`
	Action    string `gorm:"uniqueIndex:idx_permission_codename;size:20;not null"`
	Label     string `gorm:"size:150"`
	CreatedAt time.Time
}

func (Permission) TableName() string { return "permissions" }

func (p Permission) Codename() string { return Codename(p.Resource, p.Action) }

func Codename(resource, action string) string {
	return strings.ToLower(strings.TrimSpace(resource)) + "." + strings.ToLower(strings.TrimSpace(action))
}

func SplitCodename(codename string) (resource, action string, ok bool) {
	resource, action, found := strings.Cut(strings.ToLower(strings.TrimSpace(codename)), ".")
	if !found || resource == "" || action == "" {
		return "", "", false
	}
	return resource, action, true
}

func PermissionsFor(resource string) []Permission {
	out := make([]Permission, 0, len(Actions()))
	for _, action := range Actions() {
		out = append(out, Permission{
			Resource: resource,
			Action:   action,
			Label:    strings.ToUpper(action[:1]) + action[1:] + " " + resource,
		})
	}
	return out
}

type Role struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"uniqueIndex;size:100;not null"`
	Description string `gorm:"size:255"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Role) TableName() string { return "roles" }

type RolePermission struct {
	RoleID       string `gorm:"primaryKey;size:64"`
	PermissionID string `gorm:"primaryKey;size:64"`
}

func (RolePermission) TableName() string { return "role_permissions" }

type UserRole struct {
	UserID string `gorm:"primaryKey;size:64"`
	RoleID string `gorm:"primaryKey;size:64"`
}

func (UserRole) TableName() string { return "user_roles" }

type SyncReport struct {
	Created []string
	Stale   []string
}

func (r SyncReport) Empty() bool { return len(r.Created) == 0 && len(r.Stale) == 0 }

func (r *SyncReport) sort() {
	sort.Strings(r.Created)
	sort.Strings(r.Stale)
}

type PermissionStore interface {
	SyncPermissions(resources []string) (SyncReport, error)
	AllPermissions() ([]Permission, error)
	CodenamesForUser(userID string) ([]string, error)

	AllRoles() ([]Role, error)
	RoleByID(id string) (*Role, error)
	CreateRole(role *Role) error
	UpdateRole(role *Role) error
	DeleteRole(id string) error

	RolePermissions(roleID string) ([]string, error)
	SetRolePermissions(roleID string, permissionIDs []string) error
	RolesForUser(userID string) ([]string, error)
	SetUserRoles(userID string, roleIDs []string) error
}
