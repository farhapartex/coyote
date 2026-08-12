package auth

import "time"

func Prepare(u *User) {
	if u.ID == "" {
		u.ID = newUserID()
	}
	now := time.Now()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
}
