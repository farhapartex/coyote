package store

import (
	"errors"
	"time"

	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/lib/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type permissionStore struct {
	resolve Resolver
}

func Permissions(handle *gorm.DB) auth.PermissionStore {
	return LazyPermissions(func() (*gorm.DB, error) { return handle, nil })
}

func LazyPermissions(resolve Resolver) auth.PermissionStore {
	return &permissionStore{resolve: resolve}
}

func (s *permissionStore) handle() (*gorm.DB, error) {
	if s.resolve == nil {
		return nil, errors.New("coyote/store: no database resolver configured")
	}
	handle, err := s.resolve()
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, errors.New("coyote/store: no database connection")
	}
	return handle, nil
}

func (s *permissionStore) SyncPermissions(resources []string) (auth.SyncReport, error) {
	handle, err := s.handle()
	if err != nil {
		return auth.SyncReport{}, err
	}

	existing, err := s.AllPermissions()
	if err != nil {
		return auth.SyncReport{}, err
	}

	known := map[string]bool{}
	for _, permission := range existing {
		known[permission.Codename()] = true
	}
	wanted := map[string]bool{}

	report := auth.SyncReport{}
	pending := []auth.Permission{}

	for _, resource := range resources {
		for _, permission := range auth.PermissionsFor(resource) {
			wanted[permission.Codename()] = true
			if known[permission.Codename()] {
				continue
			}
			permission.ID = id.MustNew()
			permission.CreatedAt = time.Now()
			pending = append(pending, permission)
			report.Created = append(report.Created, permission.Codename())
		}
	}

	for _, permission := range existing {
		if !wanted[permission.Codename()] {
			report.Stale = append(report.Stale, permission.Codename())
		}
	}

	if len(pending) > 0 {
		if err := handle.Clauses(clause.OnConflict{DoNothing: true}).Create(&pending).Error; err != nil {
			return auth.SyncReport{}, err
		}
	}
	return report, nil
}

func (s *permissionStore) AllPermissions() ([]auth.Permission, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	rows := []auth.Permission{}
	if err := handle.Order("resource asc, action asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *permissionStore) CodenamesForUser(userID string) ([]string, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	rows := []auth.Permission{}
	err = handle.Model(&auth.Permission{}).
		Joins("join role_permissions on role_permissions.permission_id = permissions.id").
		Joins("join user_roles on user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Distinct().
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, permission := range rows {
		out = append(out, permission.Codename())
	}
	return out, nil
}

func (s *permissionStore) AllRoles() ([]auth.Role, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	rows := []auth.Role{}
	if err := handle.Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *permissionStore) RoleByID(roleID string) (*auth.Role, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	rows := []auth.Role{}
	if err := handle.Where("id = ?", roleID).Limit(1).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return &rows[0], nil
}

func (s *permissionStore) CreateRole(role *auth.Role) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	if role.ID == "" {
		role.ID = id.MustNew()
	}
	role.CreatedAt = time.Now()
	role.UpdatedAt = role.CreatedAt
	return handle.Create(role).Error
}

func (s *permissionStore) UpdateRole(role *auth.Role) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	role.UpdatedAt = time.Now()
	return handle.Save(role).Error
}

func (s *permissionStore) DeleteRole(roleID string) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	return handle.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&auth.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", roleID).Delete(&auth.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", roleID).Delete(&auth.Role{}).Error
	})
}

func (s *permissionStore) RolePermissions(roleID string) ([]string, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	rows := []auth.RolePermission{}
	if err := handle.Where("role_id = ?", roleID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.PermissionID)
	}
	return out, nil
}

func (s *permissionStore) SetRolePermissions(roleID string, permissionIDs []string) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	return handle.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&auth.RolePermission{}).Error; err != nil {
			return err
		}
		if len(permissionIDs) == 0 {
			return nil
		}
		rows := make([]auth.RolePermission, 0, len(permissionIDs))
		for _, permissionID := range permissionIDs {
			rows = append(rows, auth.RolePermission{RoleID: roleID, PermissionID: permissionID})
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}

func (s *permissionStore) RolesForUser(userID string) ([]string, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	rows := []auth.UserRole{}
	if err := handle.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.RoleID)
	}
	return out, nil
}

func (s *permissionStore) SetUserRoles(userID string, roleIDs []string) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	return handle.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&auth.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]auth.UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			rows = append(rows, auth.UserRole{UserID: userID, RoleID: roleID})
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}
