package repo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
	"gorm.io/gorm"
)

type Resolver func() (*gorm.DB, error)

type userStore struct {
	resolve Resolver
}

func Users(handle *gorm.DB) auth.Store {
	return &userStore{resolve: func() (*gorm.DB, error) { return handle, nil }}
}

func LazyUsers(resolve Resolver) auth.Store {
	return &userStore{resolve: resolve}
}

func (s *userStore) handle() (*gorm.DB, error) {
	if s.resolve == nil {
		return nil, errors.New("coyote/repo: no database resolver configured")
	}
	handle, err := s.resolve()
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, errors.New("coyote/repo: no database connection")
	}
	return handle, nil
}

func (s *userStore) ByID(id string) (*auth.User, error) {
	return s.first("id = ?", id)
}

func (s *userStore) ByUsername(username string) (*auth.User, error) {
	return s.first("lower(username) = ?", normalise(username))
}

func (s *userStore) ByEmail(email string) (*auth.User, error) {
	if normalise(email) == "" {
		return nil, auth.ErrUserNotFound
	}
	return s.first("lower(email) = ?", normalise(email))
}

func (s *userStore) first(condition string, args ...any) (*auth.User, error) {
	handle, err := s.handle()
	if err != nil {
		return nil, err
	}
	var found auth.User
	result := handle.Where(condition, args...).Limit(1).Find(&found)
	if result.Error != nil {
		return nil, fmt.Errorf("coyote/repo: reading users: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, auth.ErrUserNotFound
	}
	return &found, nil
}

func (s *userStore) Create(u *auth.User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	handle, err := s.handle()
	if err != nil {
		return err
	}
	auth.Prepare(u)
	if err := s.assertUnique(handle, u); err != nil {
		return err
	}
	if err := handle.Create(u).Error; err != nil {
		return translate(err)
	}
	return nil
}

func (s *userStore) Update(u *auth.User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	handle, err := s.handle()
	if err != nil {
		return err
	}
	if _, err := s.first("id = ?", u.ID); err != nil {
		return err
	}
	if err := s.assertUnique(handle, u); err != nil {
		return err
	}
	result := handle.Model(&auth.User{}).Where("id = ?", u.ID).Select("*").Omit("id", "created_at").Updates(u)
	if result.Error != nil {
		return translate(result.Error)
	}
	return nil
}

func (s *userStore) Delete(id string) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	result := handle.Where("id = ?", id).Delete(&auth.User{})
	if result.Error != nil {
		return fmt.Errorf("coyote/repo: deleting user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}

func (s *userStore) All() []*auth.User {
	handle, err := s.handle()
	if err != nil {
		return nil
	}
	var found []*auth.User
	if err := handle.Order("lower(username)").Find(&found).Error; err != nil {
		return nil
	}
	return found
}

func (s *userStore) Count() int {
	handle, err := s.handle()
	if err != nil {
		return 0
	}
	var total int64
	if err := handle.Model(&auth.User{}).Count(&total).Error; err != nil {
		return 0
	}
	return int(total)
}

func (s *userStore) CountActiveSuperadmins() (int, error) {
	handle, err := s.handle()
	if err != nil {
		return 0, err
	}
	var total int64
	err = handle.Model(&auth.User{}).
		Where("is_superadmin = ? AND is_active = ?", true, true).
		Count(&total).Error
	if err != nil {
		return 0, fmt.Errorf("coyote/repo: counting superadmins: %w", err)
	}
	return int(total), nil
}

func (s *userStore) assertUnique(handle *gorm.DB, u *auth.User) error {
	var clashes int64
	err := handle.Model(&auth.User{}).
		Where("lower(username) = ? AND id <> ?", normalise(u.Username), u.ID).
		Count(&clashes).Error
	if err != nil {
		return fmt.Errorf("coyote/repo: checking username: %w", err)
	}
	if clashes > 0 {
		return auth.ErrUserExists
	}
	if normalise(u.Email) == "" {
		return nil
	}
	err = handle.Model(&auth.User{}).
		Where("lower(email) = ? AND id <> ?", normalise(u.Email), u.ID).
		Count(&clashes).Error
	if err != nil {
		return fmt.Errorf("coyote/repo: checking email: %w", err)
	}
	if clashes > 0 {
		return auth.ErrEmailExists
	}
	return nil
}

func translate(err error) error {
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique") || strings.Contains(text, "duplicate") {
		if strings.Contains(text, "email") {
			return auth.ErrEmailExists
		}
		return auth.ErrUserExists
	}
	return fmt.Errorf("coyote/repo: writing user: %w", err)
}

func normalise(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
