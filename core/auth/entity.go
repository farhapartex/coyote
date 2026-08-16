package auth

import (
	"strings"
	"time"
)

type User struct {
	ID           string `gorm:"primaryKey;size:64"`
	FirstName    string
	LastName     string
	Email        string `gorm:"index;size:320"`
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Password     string `gorm:"not null"`
	IsActive     bool   `gorm:"index;default:true"`
	IsStaff      bool   `gorm:"index"`
	IsSuperadmin bool   `gorm:"index"`
	LastLoginAt  time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
}

func (u *User) DisplayName() string {
	if name := u.FullName(); name != "" {
		return name
	}
	return u.Username
}

func (u *User) Initials() string {
	first, last := strings.TrimSpace(u.FirstName), strings.TrimSpace(u.LastName)
	switch {
	case first != "" && last != "":
		return strings.ToUpper(first[:1] + last[:1])
	case first != "":
		return strings.ToUpper(first[:1])
	case last != "":
		return strings.ToUpper(last[:1])
	}
	if username := strings.TrimSpace(u.Username); username != "" {
		return strings.ToUpper(username[:1])
	}
	return "?"
}

func (u *User) CanReachAdmin() bool {
	return u != nil && u.IsActive && (u.IsStaff || u.IsSuperadmin)
}

func (u *User) HasUsablePassword() bool {
	return LooksHashed(u.Password)
}

func (u *User) HasLoggedIn() bool {
	return !u.LastLoginAt.IsZero()
}

func (u *User) Clone() *User {
	copied := *u
	return &copied
}

func (u *User) Validate() error {
	if !usernamePattern.MatchString(u.Username) {
		return ErrInvalidUser
	}
	if u.Email != "" && !emailPattern.MatchString(u.Email) {
		return ErrInvalidEmail
	}
	return nil
}
