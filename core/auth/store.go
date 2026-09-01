package auth

import "context"

type Store interface {
	ByID(ctx context.Context, id string) (*User, error)
	ByUsername(ctx context.Context, username string) (*User, error)
	ByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id string) error
	All(ctx context.Context) ([]*User, error)
	Count(ctx context.Context) (int, error)
}
