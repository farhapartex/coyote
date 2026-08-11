package auth

type Store interface {
	ByID(id string) (*User, error)
	ByUsername(username string) (*User, error)
	ByEmail(email string) (*User, error)
	Create(u *User) error
	Update(u *User) error
	Delete(id string) error
	All() []*User
	Count() int
}
