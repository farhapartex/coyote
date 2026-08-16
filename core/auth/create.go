package auth

import (
	"strings"
	"time"
)

type NewUser struct {
	Username     string
	Email        string
	FirstName    string
	LastName     string
	Password     string
	IsStaff      bool
	IsSuperadmin bool
}

func (s *Service) CreateUser(in NewUser) (*User, error) {
	if err := s.ValidatePassword(in.Password); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	u := &User{
		FirstName:    strings.TrimSpace(in.FirstName),
		LastName:     strings.TrimSpace(in.LastName),
		Email:        strings.TrimSpace(in.Email),
		Username:     strings.TrimSpace(in.Username),
		Password:     hash,
		IsActive:     true,
		IsStaff:      in.IsStaff || in.IsSuperadmin,
		IsSuperadmin: in.IsSuperadmin,
		CreatedAt:    time.Now(),
	}
	if err := s.users.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) CreateSuperadmin(username, email, password string) (*User, error) {
	return s.CreateUser(NewUser{
		Username:     username,
		Email:        email,
		Password:     password,
		IsSuperadmin: true,
	})
}

func (s *Service) SetPassword(id, password string) error {
	if err := s.ValidatePassword(password); err != nil {
		return err
	}
	u, err := s.users.ByID(id)
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	u.Password = hash
	return s.users.Update(u)
}
